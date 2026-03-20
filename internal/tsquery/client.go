package tsquery

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"tspeek/internal/config"
	"tspeek/internal/store"
)

type commandArg struct {
	Key   string
	Value string
}

// Client 是 TeamSpeak ServerQuery 协议客户端。
type Client struct {
	mu     sync.Mutex
	cfg    config.ServerQueryConfig
	logger *slog.Logger
	conn   net.Conn
	reader *bufio.Reader
}

// NewClient 创建一个新的 ServerQuery 客户端。
func NewClient(cfg config.ServerQueryConfig, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger,
	}
}

// Fetch 连接 ServerQuery 并拉取最新快照数据。
func (c *Client) Fetch(ctx context.Context, refreshInterval time.Duration, showQueryClients bool) (store.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnected(ctx); err != nil {
		return store.Snapshot{}, err
	}

	serverLines, err := c.exec(ctx, "serverinfo")
	if err != nil {
		c.closeLocked()
		return store.Snapshot{}, err
	}

	channelLines, err := c.exec(ctx, "channellist -topic -flags -limits -secondsempty")
	if err != nil {
		c.closeLocked()
		return store.Snapshot{}, err
	}

	clientLines, err := c.exec(ctx, "clientlist -uid -away -voice -times -groups -info -country")
	if err != nil {
		c.closeLocked()
		return store.Snapshot{}, err
	}

	serverRows := parseResponseRows(serverLines)
	channelRows := parseResponseRows(channelLines)
	clientRows := parseResponseRows(clientLines)

	if len(serverRows) == 0 {
		return store.Snapshot{}, fmt.Errorf("serverinfo returned no rows")
	}

	server := mapServer(serverRows[0])
	channels := mapChannels(channelRows)
	clients := mapClients(clientRows, showQueryClients)
	server.DisplayedClients = len(clients)

	return store.Snapshot{
		Server:   server,
		Channels: channels,
		Clients:  clients,
		Meta: store.SnapshotMeta{
			RefreshInterval:        refreshInterval.String(),
			RefreshIntervalSeconds: int(refreshInterval / time.Second),
		},
	}, nil
}

// Close 关闭 ServerQuery 连接。
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}
