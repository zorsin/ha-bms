package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/zorsin/ha-bms/services/bms/internal/config"
)

type httpServer struct {
	cfg    *config.Config
	mux    *http.ServeMux
	logger *slog.Logger
	server *http.Server
}

func New(cfg *config.Config, logger *slog.Logger) *httpServer {
	mux := http.NewServeMux()
	return &httpServer{
		cfg:    cfg,
		mux:    mux,
		logger: logger,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
			Handler: mux,
		},
	}
}

func (s *httpServer) RegisterHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *httpServer) ListenAndServe() error {
	s.logger.Info("Starting HTTP server", slog.Int("port", s.cfg.HTTPPort), slog.String("address", fmt.Sprintf("http://localhost:%d", s.cfg.HTTPPort)))
	return s.server.ListenAndServe()
}

func (s *httpServer) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return s.server.Shutdown(ctx)
}

func (s *httpServer) RegisterDefaultHandlers() {
	s.RegisterHandler("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	s.RegisterHandler("/read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	s.RegisterHandler("/live", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
}
