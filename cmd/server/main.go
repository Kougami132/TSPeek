package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"tspeek/internal/api"
	"tspeek/internal/config"
	"tspeek/internal/store"
	"tspeek/internal/tsquery"
)

const (
	refreshInterval  = 5 * time.Second
	showQueryClients = false

	httpReadTimeout  = 5 * time.Second
	httpWriteTimeout = 30 * time.Second
	httpIdleTimeout  = 60 * time.Second
)

type poller struct {
	client *tsquery.Client
	store  *store.SnapshotStore
	logger *slog.Logger
}

func (p *poller) run(ctx context.Context) {
	p.poll(ctx)

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	defer p.client.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *poller) poll(ctx context.Context) {
	started := time.Now()
	latest, err := p.client.Fetch(ctx)
	if err != nil {
		p.logger.Error("snapshot refresh failed", slog.Any("error", err))
		p.store.SetStale(err)
		return
	}

	latest.Meta.FetchedAt = time.Now().UTC()
	latest.Meta.LatencyMS = time.Since(started).Milliseconds()

	p.store.SetReady(latest)
}

func main() {
	defaultConfigPath := os.Getenv("TSPEEK_CONFIG")
	if defaultConfigPath == "" {
		defaultConfigPath = "config.yaml"
	}

	configPath := flag.String("config", defaultConfigPath, "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dataStore := store.New()
	queryClient := tsquery.NewClient(cfg.ServerQuery, logger)

	p := &poller{
		client: queryClient,
		store:  dataStore,
		logger: logger,
	}
	go p.run(ctx)

	apiServer := api.NewServer(api.Options{
		Logger:     logger,
		Store:      dataStore,
		ServerHost: cfg.ServerQuery.Host,
		ServerPort: cfg.ServerQuery.ServerPort,
	})

	listenAddr := ":" + strconv.Itoa(cfg.Port)
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      apiServer.Handler(),
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		queryClient.Close()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("starting server", slog.String("listen_address", listenAddr))

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
