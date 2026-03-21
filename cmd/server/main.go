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
	"tspeek/internal/icon"
	"tspeek/internal/store"
	"tspeek/internal/tsquery"
)

const (
	baseInterval     = 5 * time.Second
	maxBackoff       = 2 * time.Minute
	showQueryClients = false

	httpReadTimeout  = 5 * time.Second
	httpWriteTimeout = 30 * time.Second
	httpIdleTimeout  = 60 * time.Second
)

type poller struct {
	client           *tsquery.Client
	store            *store.SnapshotStore
	logger           *slog.Logger
	consecutiveFails int
}

func (p *poller) nextInterval() time.Duration {
	if p.consecutiveFails == 0 {
		return baseInterval
	}
	shift := p.consecutiveFails
	if shift > 5 {
		shift = 5
	}
	backoff := baseInterval * time.Duration(1<<shift)
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

func (p *poller) run(ctx context.Context) {
	p.poll(ctx)

	timer := time.NewTimer(p.nextInterval())
	defer timer.Stop()
	defer p.client.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			p.poll(ctx)
			timer.Reset(p.nextInterval())
		}
	}
}

func (p *poller) poll(ctx context.Context) {
	started := time.Now()
	latest, err := p.client.Fetch(ctx)
	if err != nil {
		p.consecutiveFails++
		interval := p.nextInterval()
		p.logger.Error("snapshot refresh failed",
			slog.Any("error", err),
			slog.String("next_retry", interval.String()),
		)
		p.store.SetStale(err)
		return
	}

	if p.consecutiveFails > 0 {
		p.logger.Info("snapshot refresh recovered",
			slog.Int("after_failures", p.consecutiveFails),
		)
	}
	p.consecutiveFails = 0

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
	iconService := icon.NewService(cfg.ServerQuery, logger)

	p := &poller{
		client: queryClient,
		store:  dataStore,
		logger: logger,
	}
	go p.run(ctx)

	apiServer := api.NewServer(api.Options{
		Logger:     logger,
		Store:      dataStore,
		Icons:      iconService,
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
