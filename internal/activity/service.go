package activity

import (
	"log/slog"
	"sync"

	"tspeek/internal/store"
)

// Service 协调 Tracker、Storage 与事件广播。
type Service struct {
	logger      *slog.Logger
	tracker     *Tracker
	storage     *Storage
	mu          sync.Mutex
	subscribers map[int]chan []Event
	nextSub     int
}

// NewService 创建 Service。
func NewService(logPath string, logger *slog.Logger) *Service {
	return &Service{
		logger:      logger,
		tracker:     NewTracker(),
		storage:     NewStorage(logPath),
		subscribers: make(map[int]chan []Event),
	}
}

// ProcessSnapshot 处理新快照，持久化事件并广播。
func (s *Service) ProcessSnapshot(snap store.Snapshot) error {
	events := s.tracker.ProcessSnapshot(snap)
	if len(events) == 0 {
		return nil
	}

	if err := s.storage.Append(events); err != nil {
		s.logger.Error("failed to append activity events", slog.Any("error", err))
		return err
	}

	s.broadcast(events)
	return nil
}

// GetActivities 分页获取活动记录。
func (s *Service) GetActivities(page, pageSize int) (PageResult, error) {
	return s.storage.ReadPage(page, pageSize)
}

// Subscribe 订阅实时产生的活动事件。
func (s *Service) Subscribe() (<-chan []Event, func()) {
	ch := make(chan []Event, 16)
	s.mu.Lock()
	id := s.nextSub
	s.nextSub++
	s.subscribers[id] = ch
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subscribers, id)
	}

	return ch, cancel
}

func (s *Service) broadcast(events []Event) {
	s.mu.Lock()
	subs := make([]chan []Event, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- events:
		default:
		}
	}
}
