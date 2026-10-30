package tsquery

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"tspeek/internal/config"
	"tspeek/internal/store"
)

const (
	permsCacheTTL    = 5 * time.Minute
	permsThrottleGap = 600 * time.Millisecond
)

// ClientOptions 配置 Client 实例。
type ClientOptions struct {
	Config            config.ServerQueryConfig
	Logger            *slog.Logger
	KeepAliveInterval time.Duration
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
}

type cmdResponse struct {
	lines []string
	err   error
}

// Client 是多路复用的 ServerQuery 客户端，维护单一持久 TCP 连接。
type Client struct {
	opts   ClientOptions
	cfg    config.ServerQueryConfig
	logger *slog.Logger

	cmdMu sync.Mutex

	stateMu   sync.RWMutex
	conn      net.Conn
	closed    bool
	connected bool
	readyCh   chan struct{}
	stopCh    chan struct{}
	closeOnce sync.Once

	inFlightMu    sync.Mutex
	inFlightChan  chan cmdResponse
	inFlightLines []string

	notifyCh chan string

	hookMu         sync.Mutex
	onConnectHooks []func(ctx context.Context) error

	lastActivityMu sync.Mutex
	lastActivity   time.Time

	permsMu          sync.Mutex
	cachedGroupPerms map[int]groupPerms
	permsCachedAt    time.Time
}

// NewClient 创建一个新的默认配置 ServerQuery 客户端。
func NewClient(cfg config.ServerQueryConfig, logger *slog.Logger) *Client {
	return NewClientWithOptions(ClientOptions{
		Config: cfg,
		Logger: logger,
	})
}

// NewClientWithOptions 创建带有定制选项的 ServerQuery 客户端。
func NewClientWithOptions(opts ClientOptions) *Client {
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

	return &Client{
		opts:         opts,
		cfg:          opts.Config,
		logger:       opts.Logger,
		readyCh:      make(chan struct{}),
		stopCh:       make(chan struct{}),
		notifyCh:     make(chan string, 1024),
		lastActivity: time.Now(),
	}
}

// Notifications 返回用于接收异步 ServerQuery 通知行的只读通道。
func (c *Client) Notifications() <-chan string {
	return c.notifyCh
}

// OnConnect 注册在每次连接或重连握手成功后调用的回调钩子。
func (c *Client) OnConnect(hook func(ctx context.Context) error) {
	c.hookMu.Lock()
	defer c.hookMu.Unlock()
	c.onConnectHooks = append(c.onConnectHooks, hook)
}

// Run 维护底层单一 ServerQuery TCP 连接并在断线时自动重试，阻塞直到 context 取消或调用 Close()。
func (c *Client) Run(ctx context.Context) {
	backoff := c.opts.InitialBackoff

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		err := c.connectAndServe(ctx)
		if ctx.Err() != nil || c.isClosed() {
			return
		}

		c.logger.Warn("serverquery connection lost, reconnecting...",
			slog.Any("error", err),
			slog.Duration("backoff", backoff),
		)

		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > c.opts.MaxBackoff {
			backoff = c.opts.MaxBackoff
		}
	}
}

func (c *Client) connectAndServe(ctx context.Context) error {
	address := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.QueryPort))
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(conn)

	// 1. 读取 Banner 头与欢迎信息
	if err := conn.SetReadDeadline(time.Now().Add(commandTimeout)); err != nil {
		_ = conn.Close()
		return err
	}
	if _, err := reader.ReadString('\n'); err != nil {
		_ = conn.Close()
		return fmt.Errorf("read header failed: %w", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		_ = conn.Close()
		return fmt.Errorf("read welcome message failed: %w", err)
	}

	// 握手前解除 ReadDeadline，由 readLoop 持续监听
	_ = conn.SetReadDeadline(time.Time{})

	c.stateMu.Lock()
	c.conn = conn
	c.stateMu.Unlock()

	readErrCh := make(chan error, 1)
	go c.readLoop(conn, reader, readErrCh)

	// 2. 串行握手：登录、选择虚拟服务器、设置昵称、注册通知
	c.cmdMu.Lock()

	loginCmd := formatCommand("login",
		commandArg{Key: "client_login_name", Value: c.cfg.Username},
		commandArg{Key: "client_login_password", Value: c.cfg.Password},
	)
	if _, err := c.execLocked(ctx, loginCmd); err != nil {
		c.cmdMu.Unlock()
		c.closeConn()
		return fmt.Errorf("login failed: %w", err)
	}

	useCmd := formatCommand("use", commandArg{Key: "port", Value: strconv.Itoa(c.cfg.ServerPort)})
	if _, err := c.execLocked(ctx, useCmd); err != nil {
		c.cmdMu.Unlock()
		c.closeConn()
		return fmt.Errorf("use failed: %w", err)
	}

	if c.cfg.Nickname != "" {
		tryUpdateNickname(c.cfg.Nickname, func(cmd string) error {
			_, err := c.execLocked(ctx, cmd)
			return err
		}, c.logger)
	}

	if _, err := c.execLocked(ctx, "servernotifyregister event=server"); err != nil {
		c.cmdMu.Unlock()
		c.closeConn()
		return fmt.Errorf("register event=server failed: %w", err)
	}

	if _, err := c.execLocked(ctx, "servernotifyregister event=channel id=0"); err != nil {
		c.cmdMu.Unlock()
		c.closeConn()
		return fmt.Errorf("register event=channel failed: %w", err)
	}

	c.cmdMu.Unlock()

	// 握手成功，标记为就绪，使后续外部指令和钩子可执行
	c.markConnected()
	defer c.markDisconnected()

	// 执行注册的 onConnect 钩子（如基准客户端列表建立）
	c.hookMu.Lock()
	hooks := make([]func(ctx context.Context) error, len(c.onConnectHooks))
	copy(hooks, c.onConnectHooks)
	c.hookMu.Unlock()

	for _, hook := range hooks {
		if err := hook(ctx); err != nil {
			c.logger.Warn("onConnect hook failed", slog.Any("error", err))
		}
	}

	// 保持连接与保活心跳
	keepAliveInterval := c.opts.KeepAliveInterval
	var ticker *time.Ticker
	var tickerC <-chan time.Time
	if keepAliveInterval > 0 {
		interval := keepAliveInterval / 2
		if interval < 20*time.Millisecond {
			interval = keepAliveInterval
		}
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
		tickerC = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			c.closeConn()
			return ctx.Err()
		case <-c.stopCh:
			c.closeConn()
			return nil
		case err := <-readErrCh:
			c.closeConn()
			return err
		case <-tickerC:
			c.lastActivityMu.Lock()
			idle := time.Since(c.lastActivity)
			c.lastActivityMu.Unlock()

			if idle >= keepAliveInterval {
				ctxTimeout, cancel := context.WithTimeout(ctx, commandTimeout)
				_, _ = c.Exec(ctxTimeout, "whoami")
				cancel()
			}
		}
	}
}

// Exec 在共享连接上执行同步 ServerQuery 命令并等待结果。
func (c *Client) Exec(ctx context.Context, cmd string) ([]string, error) {
	if err := c.waitReady(ctx); err != nil {
		return nil, err
	}

	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	return c.execLocked(ctx, cmd)
}

func (c *Client) execLocked(ctx context.Context, cmd string) ([]string, error) {
	c.stateMu.RLock()
	conn := c.conn
	c.stateMu.RUnlock()

	if conn == nil {
		return nil, errors.New("serverquery connection is not available")
	}

	resCh := make(chan cmdResponse, 1)

	c.inFlightMu.Lock()
	c.inFlightChan = resCh
	c.inFlightLines = nil
	c.inFlightMu.Unlock()

	if err := conn.SetWriteDeadline(time.Now().Add(commandTimeout)); err != nil {
		c.inFlightMu.Lock()
		c.inFlightChan = nil
		c.inFlightMu.Unlock()
		return nil, err
	}

	if _, err := io.WriteString(conn, cmd+"\n"); err != nil {
		c.inFlightMu.Lock()
		c.inFlightChan = nil
		c.inFlightMu.Unlock()
		c.closeConn()
		return nil, err
	}

	c.touchActivity()

	select {
	case <-ctx.Done():
		c.inFlightMu.Lock()
		c.inFlightChan = nil
		c.inFlightLines = nil
		c.inFlightMu.Unlock()
		c.closeConn()
		return nil, ctx.Err()
	case res := <-resCh:
		c.touchActivity()
		return res.lines, res.err
	}
}

func (c *Client) readLoop(conn net.Conn, reader *bufio.Reader, errCh chan<- error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			c.inFlightMu.Lock()
			ch := c.inFlightChan
			c.inFlightChan = nil
			c.inFlightLines = nil
			c.inFlightMu.Unlock()

			if ch != nil {
				ch <- cmdResponse{err: fmt.Errorf("connection closed: %w", err)}
			}
			select {
			case errCh <- err:
			default:
			}
			return
		}

		line = strings.Trim(line, "\r\n")
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "notify") {
			select {
			case c.notifyCh <- line:
			default:
				c.logger.Warn("notify channel full, dropping line", slog.String("line", line))
			}
			continue
		}

		c.inFlightMu.Lock()
		ch := c.inFlightChan
		if strings.HasPrefix(line, "error ") {
			lines := c.inFlightLines
			c.inFlightChan = nil
			c.inFlightLines = nil
			c.inFlightMu.Unlock()

			if ch != nil {
				err := parseErrorLine(line)
				ch <- cmdResponse{lines: lines, err: err}
			} else {
				c.logger.Warn("unexpected error line received with no in-flight command", slog.String("line", line))
			}
			continue
		}

		if ch != nil {
			c.inFlightLines = append(c.inFlightLines, line)
			c.inFlightMu.Unlock()
		} else {
			c.inFlightMu.Unlock()
			c.logger.Warn("unexpected data line received with no in-flight command", slog.String("line", line))
		}
	}
}

func (c *Client) markConnected() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if !c.connected {
		c.connected = true
		close(c.readyCh)
	}
}

func (c *Client) markDisconnected() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.connected {
		c.connected = false
		c.readyCh = make(chan struct{})
	}
}

func (c *Client) waitReady(ctx context.Context) error {
	c.stateMu.RLock()
	if c.connected {
		c.stateMu.RUnlock()
		return nil
	}
	readyCh := c.readyCh
	c.stateMu.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stopCh:
		return errors.New("client closed")
	case <-readyCh:
		return nil
	}
}

func (c *Client) touchActivity() {
	c.lastActivityMu.Lock()
	c.lastActivity = time.Now()
	c.lastActivityMu.Unlock()
}

func (c *Client) closeConn() {
	c.stateMu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.stateMu.Unlock()
}

func (c *Client) isClosed() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.closed
}

// Close 关闭 ServerQuery 客户端会话。
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.stateMu.Lock()
		c.closed = true
		c.stateMu.Unlock()
		close(c.stopCh)
		c.closeConn()
	})
}

// Fetch 拉取虚拟服务器最新快照数据。
func (c *Client) Fetch(ctx context.Context) (store.Snapshot, error) {
	serverLines, err := c.Exec(ctx, "serverinfo")
	if err != nil {
		return store.Snapshot{}, err
	}

	channelLines, err := c.Exec(ctx, "channellist -topic -flags -limits -secondsempty")
	if err != nil {
		return store.Snapshot{}, err
	}

	clientLines, err := c.Exec(ctx, "clientlist -uid -away -voice -times -groups -info -country")
	if err != nil {
		return store.Snapshot{}, err
	}

	groupLines, err := c.Exec(ctx, "servergrouplist")
	if err != nil {
		return store.Snapshot{}, err
	}

	channelGroupLines, err := c.Exec(ctx, "channelgrouplist")
	if err != nil {
		return store.Snapshot{}, err
	}

	serverRows := parseResponseRows(serverLines)
	channelRows := parseResponseRows(channelLines)
	clientRows := parseResponseRows(clientLines)
	groupRows := parseResponseRows(groupLines)
	channelGroupRows := parseResponseRows(channelGroupLines)

	if len(serverRows) == 0 {
		return store.Snapshot{}, fmt.Errorf("serverinfo returned no rows")
	}

	allGroupPerms := c.resolvePermsCache(ctx, groupRows)

	server := mapServer(serverRows[0])
	channels := mapChannels(channelRows)
	clients := mapClients(clientRows, allGroupPerms)
	server.DisplayedClients = len(clients)
	serverGroups := mapServerGroups(groupRows, allGroupPerms)
	channelGroups := mapChannelGroups(channelGroupRows)

	return store.Snapshot{
		Server:        server,
		Channels:      channels,
		Clients:       clients,
		ServerGroups:  serverGroups,
		ChannelGroups: channelGroups,
	}, nil
}

func (c *Client) resolvePermsCache(ctx context.Context, groupRows []map[string]string) map[int]groupPerms {
	c.permsMu.Lock()
	cacheValid := c.cachedGroupPerms != nil && time.Since(c.permsCachedAt) < permsCacheTTL
	if cacheValid {
		cached := c.cachedGroupPerms
		c.permsMu.Unlock()
		return cached
	}
	c.permsMu.Unlock()

	perms := make(map[int]groupPerms, len(groupRows))
	for i, row := range groupRows {
		if i > 0 {
			select {
			case <-ctx.Done():
				c.permsMu.Lock()
				defer c.permsMu.Unlock()
				if c.cachedGroupPerms != nil {
					return c.cachedGroupPerms
				}
				return perms
			case <-time.After(permsThrottleGap):
			}
		}

		sgid := parseInt(row["sgid"])
		permLines, err := c.Exec(ctx, fmt.Sprintf("servergrouppermlist sgid=%d -permsid", sgid))
		if err != nil {
			continue
		}
		permRows := parseResponseRows(permLines)
		perms[sgid] = mapGroupPerms(permRows)
	}

	c.permsMu.Lock()
	c.cachedGroupPerms = perms
	c.permsCachedAt = time.Now()
	c.permsMu.Unlock()

	c.logger.Debug("group permissions cache refreshed", slog.Int("groups", len(perms)))
	return perms
}
