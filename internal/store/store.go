package store

import "sync"

// PublicUpstreamError 是暴露给前端的通用上游错误消息。
const PublicUpstreamError = "upstream_unavailable"

// SnapshotStore 管理当前快照的内存状态与订阅发布。
type SnapshotStore struct {
	mu             sync.RWMutex
	snapshot       *Snapshot
	lastError      string
	sequence       uint64
	subscribers    map[int]chan Snapshot
	nextSubscriber int
}

// New 创建一个新的 SnapshotStore。
func New() *SnapshotStore {
	return &SnapshotStore{
		subscribers: make(map[int]chan Snapshot),
	}
}

// Ready 返回是否已产生至少一个有效快照。
func (s *SnapshotStore) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot != nil
}

// LastErr 返回最近一次轮询错误消息。
func (s *SnapshotStore) LastErr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Current 返回当前快照的副本。若尚未就绪则 ok 为 false。
func (s *SnapshotStore) Current() (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot == nil {
		return Snapshot{}, false
	}
	return cloneSnapshot(*s.snapshot), true
}

// SetReady 设置一个新的成功快照并广播给所有订阅者。
func (s *SnapshotStore) SetReady(next Snapshot) {
	s.mu.Lock()
	s.sequence++
	next.Meta.Sequence = s.sequence
	next.Meta.Stale = false
	next.Meta.LastError = ""
	snapshotCopy := cloneSnapshot(next)
	s.snapshot = &snapshotCopy
	s.lastError = ""
	subs := s.copySubscribersLocked()
	s.mu.Unlock()
	broadcastSnapshot(subs, snapshotCopy)
}

// SetStale 标记当前快照为 stale 并广播错误状态。
func (s *SnapshotStore) SetStale(err error) {
	s.mu.Lock()
	s.lastError = err.Error()
	if s.snapshot == nil {
		s.mu.Unlock()
		return
	}

	s.sequence++
	stale := cloneSnapshot(*s.snapshot)
	stale.Meta.Sequence = s.sequence
	stale.Meta.Stale = true
	stale.Meta.LastError = PublicUpstreamError
	s.snapshot = &stale
	subs := s.copySubscribersLocked()
	s.mu.Unlock()
	broadcastSnapshot(subs, stale)
}

// Subscribe 返回一个快照更新 channel 和取消函数。
func (s *SnapshotStore) Subscribe() (<-chan Snapshot, func()) {
	ch := make(chan Snapshot, 1)
	s.mu.Lock()
	id := s.nextSubscriber
	s.nextSubscriber++
	s.subscribers[id] = ch
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
		}
	}

	return ch, cancel
}

func (s *SnapshotStore) copySubscribersLocked() []chan Snapshot {
	result := make([]chan Snapshot, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		result = append(result, subscriber)
	}
	return result
}

func broadcastSnapshot(subscribers []chan Snapshot, payload Snapshot) {
	for _, subscriber := range subscribers {
		select {
		case subscriber <- cloneSnapshot(payload):
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- cloneSnapshot(payload):
			default:
			}
		}
	}
}

func cloneSnapshot(src Snapshot) Snapshot {
	cloned := src
	if src.Channels != nil {
		cloned.Channels = make([]ChannelInfo, len(src.Channels))
		copy(cloned.Channels, src.Channels)
	} else {
		cloned.Channels = []ChannelInfo{}
	}
	if src.Clients != nil {
		cloned.Clients = make([]ClientInfo, len(src.Clients))
		copy(cloned.Clients, src.Clients)
	} else {
		cloned.Clients = []ClientInfo{}
	}
	return cloned
}
