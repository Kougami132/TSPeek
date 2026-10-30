package tsquery

import (
	"context"
	"fmt"
	"log/slog"
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
	Client            *Client
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

// EventListener 监听原生 ServerQuery 事件推送并转换为 Activity Event。
type EventListener struct {
	opts       EventListenerOptions
	client     *Client
	ownsClient bool
	logger     *slog.Logger
	resolver   ChannelResolver
	recorder   ActivityRecorder

	mu        sync.Mutex
	clients   map[int]clientSession
	stopCh    chan struct{}
	closeOnce sync.Once
}

// NewEventListener 创建一个新的事件监听器实例。
func NewEventListener(opts EventListenerOptions) *EventListener {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	ownsClient := false
	if opts.Client == nil {
		opts.Client = NewClientWithOptions(ClientOptions{
			Config:            opts.Config,
			Logger:            opts.Logger,
			KeepAliveInterval: opts.KeepAliveInterval,
			InitialBackoff:    opts.InitialBackoff,
			MaxBackoff:        opts.MaxBackoff,
		})
		ownsClient = true
	}

	l := &EventListener{
		opts:       opts,
		client:     opts.Client,
		ownsClient: ownsClient,
		logger:     opts.Logger,
		resolver:   opts.Resolver,
		recorder:   opts.Recorder,
		clients:    make(map[int]clientSession),
		stopCh:     make(chan struct{}),
	}

	// 注册重连基准钩子：连接/重连建立后，静默拉取 clientlist 建立当前在线会话映射
	l.client.OnConnect(func(ctx context.Context) error {
		lines, err := l.client.Exec(ctx, "clientlist -uid")
		if err != nil {
			return fmt.Errorf("baseline clientlist failed: %w", err)
		}
		l.populateBaseline(lines)
		return nil
	})

	return l
}

// Run 启动监听消费循环。若自行拥有底层 Client，则同时在后台运行 Client 连接管理。
func (l *EventListener) Run(ctx context.Context) {
	if l.ownsClient {
		go l.client.Run(ctx)
	}

	notifyCh := l.client.Notifications()

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case line, ok := <-notifyCh:
			if !ok {
				return
			}
			l.handleLine(line)
		}
	}
}

// Close 关闭监听器。
func (l *EventListener) Close() {
	l.closeOnce.Do(func() {
		close(l.stopCh)
		if l.ownsClient {
			l.client.Close()
		}
	})
}

func (l *EventListener) populateBaseline(lines []string) {
	rows := parseResponseRows(lines)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clients = make(map[int]clientSession, len(rows))
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
	l.logger.Info("event listener baselined successfully", slog.Int("online_clients", len(l.clients)))
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
