package tsquery

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"tspeek/internal/activity"
	"tspeek/internal/config"
)

// ChannelResolver 解析频道 ID 对应的名称。
type ChannelResolver interface {
	ResolveChannelName(id int) string
}

// ActivityRecorder 接收生成的活动事件。
type ActivityRecorder interface {
	RecordEvents(events []activity.Event) error
}

// EventListenerOptions 配置 EventListener。
type EventListenerOptions struct {
	Config            config.ServerQueryConfig
	Logger            *slog.Logger
	Resolver          ChannelResolver
	Recorder          ActivityRecorder
	KeepAliveInterval time.Duration
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
}

type clientSession struct {
	clid       int
	uid        string
	nickname   string
	channelID  int
	clientType int
}

// EventListener 维护专用的持久 ServerQuery 连接，监听原生事件推送并转换为 Activity Event。
type EventListener struct {
	opts     EventListenerOptions
	logger   *slog.Logger
	resolver ChannelResolver
	recorder ActivityRecorder

	mu        sync.Mutex
	writeMu   sync.Mutex
	conn      net.Conn
	reader    *bufio.Reader
	clients   map[int]clientSession
	stopCh    chan struct{}
	closed    bool
	closeOnce sync.Once
}

// NewEventListener 创建一个新的事件监听器实例。
func NewEventListener(opts EventListenerOptions) *EventListener {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.KeepAliveInterval <= 0 {
		opts.KeepAliveInterval = 60 * time.Second
	}
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = 1 * time.Second
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 60 * time.Second
	}

	return &EventListener{
		opts:     opts,
		logger:   opts.Logger,
		resolver: opts.Resolver,
		recorder: opts.Recorder,
		clients:  make(map[int]clientSession),
		stopCh:   make(chan struct{}),
	}
}

// Run 启动监听循环并在网络断开时自动重连。阻塞直到 context 取消或调用 Close()。
func (l *EventListener) Run(ctx context.Context) {
	backoff := l.opts.InitialBackoff

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		default:
		}

		err := l.connectAndListen(ctx)
		if ctx.Err() != nil || l.isClosed() {
			return
		}

		l.logger.Warn("event listener disconnected, reconnecting...",
			slog.Any("error", err),
			slog.Duration("backoff", backoff),
		)

		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > l.opts.MaxBackoff {
			backoff = l.opts.MaxBackoff
		}
	}
}

func (l *EventListener) connectAndListen(ctx context.Context) error {
	address := net.JoinHostPort(l.opts.Config.Host, strconv.Itoa(l.opts.Config.QueryPort))
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.conn = conn
	l.reader = bufio.NewReader(conn)
	// 清空历史客户端缓存，准备重新基准初始化
	l.clients = make(map[int]clientSession)
	l.mu.Unlock()

	defer l.closeConn()

	// 1. 读取 Banner 头与欢迎信息
	if _, err := l.readLine(); err != nil {
		return fmt.Errorf("failed to read banner: %w", err)
	}
	if _, err := l.readLine(); err != nil {
		return fmt.Errorf("failed to read welcome message: %w", err)
	}

	// 2. 认证登录
	loginCmd := formatCommand("login",
		commandArg{Key: "client_login_name", Value: l.opts.Config.Username},
		commandArg{Key: "client_login_password", Value: l.opts.Config.Password},
	)
	if _, err := l.execSync(loginCmd); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// 3. 选择虚拟服务器
	useCmd := formatCommand("use", commandArg{Key: "port", Value: strconv.Itoa(l.opts.Config.ServerPort)})
	if _, err := l.execSync(useCmd); err != nil {
		return fmt.Errorf("use port failed: %w", err)
	}

	// 4. 注册服务器级与频道级事件通知
	if _, err := l.execSync("servernotifyregister event=server"); err != nil {
		return fmt.Errorf("register event=server failed: %w", err)
	}
	if _, err := l.execSync("servernotifyregister event=channel id=0"); err != nil {
		return fmt.Errorf("register event=channel failed: %w", err)
	}

	// 5. 拉取初始 clientlist 静默建立基准
	clientLines, err := l.execSync("clientlist -uid")
	if err != nil {
		return fmt.Errorf("baseline clientlist failed: %w", err)
	}
	l.populateBaseline(clientLines)

	l.logger.Info("event listener connected and baselined successfully",
		slog.Int("online_clients", len(l.clients)),
	)

	// 6. 启动后台保活心跳
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)

	go func() {
		ticker := time.NewTicker(l.opts.KeepAliveInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-l.stopCh:
				return
			case <-heartbeatDone:
				return
			case <-ticker.C:
				if err := l.sendKeepAlive(); err != nil {
					l.logger.Debug("keepalive ping failed", slog.Any("error", err))
					l.closeConn()
					return
				}
			}
		}
	}()

	// 7. 持续读取事件流
	for {
		line, err := l.readStreamLine()
		if err != nil {
			return err
		}
		if line == "" {
			continue
		}
		l.handleLine(line)
	}
}

func (l *EventListener) sendKeepAlive() error {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()

	l.mu.Lock()
	conn := l.conn
	l.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("connection not available")
	}

	if err := conn.SetWriteDeadline(time.Now().Add(commandTimeout)); err != nil {
		return err
	}
	_, err := io.WriteString(conn, "whoami\n")
	return err
}

func (l *EventListener) readStreamLine() (string, error) {
	l.mu.Lock()
	conn := l.conn
	reader := l.reader
	l.mu.Unlock()

	if conn == nil || reader == nil {
		return "", io.EOF
	}

	timeout := l.opts.KeepAliveInterval * 3
	if timeout < 2*time.Second {
		timeout = 2 * time.Second
	}
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.Trim(line, "\r\n"), nil
}

func (l *EventListener) populateBaseline(lines []string) {
	rows := parseResponseRows(lines)
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, row := range rows {
		clid := parseInt(row["clid"])
		if clid == 0 {
			continue
		}
		l.clients[clid] = clientSession{
			clid:       clid,
			uid:        row["client_unique_identifier"],
			nickname:   row["client_nickname"],
			channelID:  parseInt(row["cid"]),
			clientType: parseInt(row["client_type"]),
		}
	}
}

func (l *EventListener) handleLine(line string) {
	switch {
	case strings.HasPrefix(line, "notifycliententerview"):
		l.handleClientEnter(strings.TrimPrefix(line, "notifycliententerview"))
	case strings.HasPrefix(line, "notifyclientleftview"):
		l.handleClientLeft(strings.TrimPrefix(line, "notifyclientleftview"))
	case strings.HasPrefix(line, "notifyclientmoved"):
		l.handleClientMoved(strings.TrimPrefix(line, "notifyclientmoved"))
	case strings.HasPrefix(line, "notifyclientupdated"):
		l.handleClientUpdated(strings.TrimPrefix(line, "notifyclientupdated"))
	case strings.HasPrefix(line, "error "):
		if err := parseErrorLine(line); err != nil {
			l.logger.Warn("serverquery error notification received", slog.Any("error", err))
		}
	}
}

func (l *EventListener) handleClientEnter(body string) {
	rows := parseResponseRows([]string{body})
	var events []activity.Event

	l.mu.Lock()
	for _, row := range rows {
		clid := parseInt(row["clid"])
		if clid == 0 {
			continue
		}
		uid := row["client_unique_identifier"]
		nick := row["client_nickname"]
		ctid := parseInt(row["ctid"])
		cType := parseInt(row["client_type"])

		l.clients[clid] = clientSession{
			clid:       clid,
			uid:        uid,
			nickname:   nick,
			channelID:  ctid,
			clientType: cType,
		}

		if cType != 0 {
			continue
		}

		chName := l.resolveChannelName(ctid)
		events = append(events, activity.Event{
			Action:      "join",
			UID:         uid,
			Nickname:    nick,
			ChannelID:   ctid,
			ChannelName: chName,
		})
	}
	l.mu.Unlock()

	l.emitEvents(events)
}

func (l *EventListener) handleClientLeft(body string) {
	rows := parseResponseRows([]string{body})
	var events []activity.Event

	l.mu.Lock()
	for _, row := range rows {
		clid := parseInt(row["clid"])
		if clid == 0 {
			continue
		}

		session, exists := l.clients[clid]
		delete(l.clients, clid)

		if !exists || session.clientType != 0 {
			continue
		}

		chName := l.resolveChannelName(session.channelID)
		events = append(events, activity.Event{
			Action:      "leave",
			UID:         session.uid,
			Nickname:    session.nickname,
			ChannelID:   session.channelID,
			ChannelName: chName,
		})
	}
	l.mu.Unlock()

	l.emitEvents(events)
}

func (l *EventListener) handleClientMoved(body string) {
	rows := parseResponseRows([]string{body})
	var events []activity.Event

	l.mu.Lock()
	for _, row := range rows {
		clid := parseInt(row["clid"])
		if clid == 0 {
			continue
		}

		session, exists := l.clients[clid]
		if !exists || session.clientType != 0 {
			continue
		}

		targetChannelID := parseInt(row["ctid"])
		fromChannelID := session.channelID
		session.channelID = targetChannelID
		l.clients[clid] = session

		fromChName := l.resolveChannelName(fromChannelID)
		targetChName := l.resolveChannelName(targetChannelID)

		events = append(events, activity.Event{
			Action:          "move",
			UID:             session.uid,
			Nickname:        session.nickname,
			ChannelID:       targetChannelID,
			ChannelName:     targetChName,
			FromChannelID:   fromChannelID,
			FromChannelName: fromChName,
		})
	}
	l.mu.Unlock()

	l.emitEvents(events)
}

func (l *EventListener) handleClientUpdated(body string) {
	rows := parseResponseRows([]string{body})
	var events []activity.Event

	l.mu.Lock()
	for _, row := range rows {
		clid := parseInt(row["clid"])
		if clid == 0 {
			continue
		}

		session, exists := l.clients[clid]
		if !exists || session.clientType != 0 {
			continue
		}

		newNick, hasNick := row["client_nickname"]
		if !hasNick || newNick == session.nickname {
			continue
		}

		oldNick := session.nickname
		session.nickname = newNick
		l.clients[clid] = session

		chName := l.resolveChannelName(session.channelID)

		events = append(events, activity.Event{
			Action:         "rename",
			UID:            session.uid,
			Nickname:       oldNick,
			TargetNickname: newNick,
			ChannelID:      session.channelID,
			ChannelName:    chName,
		})
	}
	l.mu.Unlock()

	l.emitEvents(events)
}

func (l *EventListener) resolveChannelName(id int) string {
	if l.resolver != nil {
		name := l.resolver.ResolveChannelName(id)
		if name != "" {
			return name
		}
	}
	return fmt.Sprintf("Channel #%d", id)
}

func (l *EventListener) emitEvents(events []activity.Event) {
	if len(events) == 0 || l.recorder == nil {
		return
	}
	if err := l.recorder.RecordEvents(events); err != nil {
		l.logger.Error("failed to record activity events", slog.Any("error", err))
	}
}

func (l *EventListener) execSync(command string) ([]string, error) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()

	l.mu.Lock()
	conn := l.conn
	l.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("connection not available")
	}

	if err := conn.SetWriteDeadline(time.Now().Add(commandTimeout)); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(conn, command+"\n"); err != nil {
		return nil, err
	}

	lines := make([]string, 0, 4)
	for {
		line, err := l.readLine()
		if err != nil {
			return nil, err
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "notify") {
			// 在握手期间可能收到早期通知，丢弃或稍后在基准同步
			continue
		}
		if strings.HasPrefix(line, "error ") {
			if err := parseErrorLine(line); err != nil {
				return nil, err
			}
			return lines, nil
		}
		lines = append(lines, line)
	}
}

func (l *EventListener) readLine() (string, error) {
	l.mu.Lock()
	reader := l.reader
	conn := l.conn
	l.mu.Unlock()

	if conn == nil || reader == nil {
		return "", io.EOF
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.Trim(line, "\r\n"), nil
}

func (l *EventListener) closeConn() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		_ = l.conn.Close()
		l.conn = nil
		l.reader = nil
	}
}

func (l *EventListener) isClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

// Close 关闭监听器并断开连接。
func (l *EventListener) Close() {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		l.mu.Unlock()
		close(l.stopCh)
		l.closeConn()
	})
}
