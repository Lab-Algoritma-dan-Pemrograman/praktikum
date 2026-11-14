package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/application"
	"github.com/algohub/algohub-rewrite/backend/internal/config"
)

func listenAddress(fallback, port string) (string, error) {
	if port == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "", errors.New("PORT must be an integer between 1 and 65535")
	}
	return net.JoinHostPort("", strconv.Itoa(value)), nil
}

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr, err := listenAddress(cfg.Addr, os.Getenv("PORT"))
	if err != nil {
		logger.Error("API listener configuration rejected", "error", err)
		os.Exit(1)
	}
	cfg.Addr = addr
	app, err := application.New(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("API configuration rejected", "error", err)
		os.Exit(1)
	}
	defer app.Close()
	server := &http.Server{Addr: cfg.Addr, Handler: app.Handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: cfg.RequestTimeout + 5*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failed := make(chan error, 1)
	go func() {
		logger.Info("API started", "addr", cfg.Addr, "environment", cfg.Environment, "fixtures", cfg.EnableFixtures)
		failed <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err := <-failed:
		if err != http.ErrServerClosed {
			logger.Error("API stopped unexpectedly", "error", err)
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		logger.Error("API shutdown failed", "error", err)
	}
}
