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

	"github.com/zorsin/ha-bms/internal/logger"
	"github.com/zorsin/ha-bms/services/bms/internal/config"
	"github.com/zorsin/ha-bms/services/bms/internal/httpserver"
	"github.com/zorsin/ha-bms/services/bms/internal/service"
)

func main() {
	if err := run(); err != nil {
		log := logger.Get()
		log.Error("app stopped with error", slog.Any("err", err))
	}
}

func run() error {
	cfg := config.Get()
	if cfg == nil {
		return errors.New("failed to load config")
	}
	log := logger.Get(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	httpServer := httpserver.New(cfg, log)

	httpServer.RegisterDefaultHandlers()

	service := service.New(cfg, log)
	service.Start(ctx)

	for pattern, handler := range service.GetHTTPHandlers() {
		httpServer.RegisterHandler(pattern, handler)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		service.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serverErr:
		service.Shutdown()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
