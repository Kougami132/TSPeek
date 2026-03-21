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

const (
	permsCacheTTL    = 5 * time.Minute
	permsThrottleGap = 600 * time.Millisecond
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

	cachedGroupPerms map[int]groupPerms
	permsCachedAt    time.Time
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

	reconnected, err := c.ensureConnected(ctx)
	if err != nil {
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

	channelGroupLines, err := c.exec(ctx, "channelgrouplist")
	if err != nil {
		c.closeLocked()
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

	allGroupPerms := c.resolvePermsCache(ctx, reconnected, groupRows)

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

// resolvePermsCache returns group permissions, using cache when possible.
// Skips refresh on reconnection to keep command count low; throttles
// individual queries when actually refreshing.
func (c *Client) resolvePermsCache(ctx context.Context, reconnected bool, groupRows []map[string]string) map[int]groupPerms {
	cacheValid := c.cachedGroupPerms != nil && time.Since(c.permsCachedAt) < permsCacheTTL

	if reconnected && c.cachedGroupPerms != nil {
		return c.cachedGroupPerms
	}
	if cacheValid {
		return c.cachedGroupPerms
	}

	perms := make(map[int]groupPerms, len(groupRows))
	for i, row := range groupRows {
		if i > 0 {
			select {
			case <-ctx.Done():
				if c.cachedGroupPerms != nil {
					return c.cachedGroupPerms
				}
				return perms
			case <-time.After(permsThrottleGap):
			}
		}

		sgid := parseInt(row["sgid"])
		permLines, err := c.exec(ctx, fmt.Sprintf("servergrouppermlist sgid=%d -permsid", sgid))
		if err != nil {
			continue
		}
		permRows := parseResponseRows(permLines)
		perms[sgid] = mapGroupPerms(permRows)
	}

	c.cachedGroupPerms = perms
	c.permsCachedAt = time.Now()
	c.logger.Debug("group permissions cache refreshed", slog.Int("groups", len(perms)))
	return perms
}

// Close 关闭 ServerQuery 连接。
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}
