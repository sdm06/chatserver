package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/chatserver/internal/config"
	"example.com/chatserver/internal/hub"
	"example.com/chatserver/internal/server"
	"example.com/chatserver/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

	if cfg.AuthToken == "" && !cfg.Dev {
		log.Warn("CHAT_AUTH_TOKEN is empty; server accepts unauthenticated clients (set CHAT_DEV=true to silence)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := hub.New(cfg.HistorySize)
	go h.Run(ctx)

	srv := server.New(cfg, h, log)
	if err := srv.Listen(); err != nil {
		log.Error("tcp listen failed", "err", err)
		os.Exit(1)
	}
	log.Info("chat server listening", "addr", srv.Addr(), "tls", cfg.TLSEnabled())

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           web.New(h, cfg, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
		}
	}()
	log.Info("web dashboard listening", "addr", cfg.HTTPAddr)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sig:
		log.Info("shutting down", "signal", s.String())
	case err := <-serveErr:
		if err != nil {
			log.Error("serve error", "err", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = srv.Shutdown(shutdownCtx)
	log.Info("bye")
}
