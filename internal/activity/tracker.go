package activity

import (
	"sync"
	"time"

	"tspeek/internal/store"
)

type cachedClient struct {
	UID       string
	Nickname  string
	ChannelID int
}

// Tracker 对比连续的 Snapshot 并产生活动事件。
type Tracker struct {
	mu          sync.Mutex
	initialized bool
	clients     map[string]cachedClient // key: UID
	channels    map[int]string          // channel_id -> channel_name
	lastID      int64
}

// NewTracker 创建 Tracker 实例。
func NewTracker() *Tracker {
	return &Tracker{
		clients:  make(map[string]cachedClient),
		channels: make(map[int]string),
	}
}

func (t *Tracker) nextID() int64 {
	nowMicros := time.Now().UnixMicro()
	if nowMicros <= t.lastID {
		nowMicros = t.lastID + 1
	}
	t.lastID = nowMicros
	return nowMicros
}

// ProcessSnapshot 对比新快照与之前状态，返回产生的新活动事件。
// 首次调用（冷启动）仅建立基准数据，不产生任何事件。
func (t *Tracker) ProcessSnapshot(snap store.Snapshot) []Event {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 1. 构建当前频道映射
	currChannels := make(map[int]string, len(snap.Channels))
	for _, ch := range snap.Channels {
		currChannels[ch.ID] = ch.Name
	}

	// 2. 过滤仅保留普通语音客户端（忽略 ServerQuery 等 client_type != 0）
	currClients := make(map[string]cachedClient)
	for _, c := range snap.Clients {
		if c.Type != 0 {
			continue
		}
		uid := c.UniqueID
		if uid == "" {
			// 若无 UID 则跳过
			continue
		}
		currClients[uid] = cachedClient{
			UID:       uid,
			Nickname:  c.Nickname,
			ChannelID: c.ChannelID,
		}
	}

	// 3. 首次快照作为基准静默初始化
	if !t.initialized {
		t.clients = currClients
		t.channels = currChannels
		t.initialized = true
		return nil
	}

	now := time.Now().UTC()
	var events []Event

	// 4. 检查离线事件（在前一个快照中存在，当前快照中不存在）
	for uid, prev := range t.clients {
		if _, exists := currClients[uid]; !exists {
			chName := t.channels[prev.ChannelID]
			if name, ok := currChannels[prev.ChannelID]; ok {
				chName = name
			}
			events = append(events, Event{
				ID:          t.nextID(),
				Time:        now,
				Action:      "leave",
				UID:         uid,
				Nickname:    prev.Nickname,
				ChannelID:   prev.ChannelID,
				ChannelName: chName,
			})
		}
	}

	// 5. 检查加入、改名与频道切换事件
	for uid, curr := range currClients {
		prev, exists := t.clients[uid]
		currChName := currChannels[curr.ChannelID]

		if !exists {
			// 加入事件
			events = append(events, Event{
				ID:          t.nextID(),
				Time:        now,
				Action:      "join",
				UID:         uid,
				Nickname:    curr.Nickname,
				ChannelID:   curr.ChannelID,
				ChannelName: currChName,
			})
		} else {
			// 存在客户端：检查改名
			if prev.Nickname != curr.Nickname {
				events = append(events, Event{
					ID:             t.nextID(),
					Time:           now,
					Action:         "rename",
					UID:            uid,
					Nickname:       prev.Nickname,
					TargetNickname: curr.Nickname,
					ChannelID:      curr.ChannelID,
					ChannelName:    currChName,
				})
			}
			// 检查频道切换
			if prev.ChannelID != curr.ChannelID {
				fromChName := t.channels[prev.ChannelID]
				if name, ok := currChannels[prev.ChannelID]; ok {
					fromChName = name
				}
				events = append(events, Event{
					ID:              t.nextID(),
					Time:            now,
					Action:          "move",
					UID:             uid,
					Nickname:        curr.Nickname,
					ChannelID:       curr.ChannelID,
					ChannelName:     currChName,
					FromChannelID:   prev.ChannelID,
					FromChannelName: fromChName,
				})
			}
		}
	}

	// 6. 更新基准状态
	t.clients = currClients
	t.channels = currChannels

	return events
}
