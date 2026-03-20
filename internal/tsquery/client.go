package tsquery

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

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
func (c *Client) Fetch(ctx context.Context) (store.Snapshot, error) {
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

	groupLines, err := c.exec(ctx, "servergrouplist")
	if err != nil {
		c.closeLocked()
		return store.Snapshot{}, err
	}

	serverRows := parseResponseRows(serverLines)
	channelRows := parseResponseRows(channelLines)
	clientRows := parseResponseRows(clientLines)
	groupRows := parseResponseRows(groupLines)

	if len(serverRows) == 0 {
		return store.Snapshot{}, fmt.Errorf("serverinfo returned no rows")
	}

	// 获取每个服务器组的权限（i_client_talk_power, i_group_sort_id）
	allGroupPerms := make(map[int]groupPerms, len(groupRows))
	for _, row := range groupRows {
		sgid := parseInt(row["sgid"])
		permLines, err := c.exec(ctx, fmt.Sprintf("servergrouppermlist sgid=%d -permsid", sgid))
		if err != nil {
			// 权限不足时跳过该组，不中断连接
			continue
		}
		permRows := parseResponseRows(permLines)
		allGroupPerms[sgid] = mapGroupPerms(permRows)
	}

	server := mapServer(serverRows[0])
	channels := mapChannels(channelRows)
	clients := mapClients(clientRows, allGroupPerms)
	server.DisplayedClients = len(clients)

	return store.Snapshot{
		Server:   server,
		Channels: channels,
		Clients:  clients,
	}, nil
}

// Close 关闭 ServerQuery 连接。
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}
