package api

import (
	"log/slog"
	"net/http"
	"time"

	"tspeek/internal/store"
)

// SnapshotSource 定义了 API 层对快照存储的最小接口。
type SnapshotSource interface {
	Ready() bool
	Current() (store.Snapshot, bool)
	Subscribe() (<-chan store.Snapshot, func())
}

// Server 是 HTTP API 服务器。
type Server struct {
	logger           *slog.Logger
	store            SnapshotSource
	refreshInterval  time.Duration
	showQueryClients bool
	serverHost       string
	serverPort       int
}

// Options 是创建 Server 所需的选项。
type Options struct {
	Logger           *slog.Logger
	Store            SnapshotSource
	RefreshInterval  time.Duration
	ShowQueryClients bool
	ServerHost       string
	ServerPort       int
}

// NewServer 创建一个新的 API Server。
func NewServer(opts Options) *Server {
	return &Server{
		logger:           opts.Logger,
		store:            opts.Store,
		refreshInterval:  opts.RefreshInterval,
		showQueryClients: opts.ShowQueryClients,
		serverHost:       opts.ServerHost,
		serverPort:       opts.ServerPort,
	}
}

// Handler 返回完整的 HTTP Handler（含中间件）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)
	return loggingMiddleware(s.logger, mux)
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/api/v1/public-config", s.handlePublicConfig)
	mux.HandleFunc("/api/v1/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/v1/stream", s.handleStream)

	staticHandler, err := newStaticHandler()
	if err != nil {
		s.logger.Error("failed to initialize static handler", slog.Any("error", err))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "static assets unavailable", http.StatusInternalServerError)
		})
		return
	}
	mux.Handle("/", staticHandler)
}
