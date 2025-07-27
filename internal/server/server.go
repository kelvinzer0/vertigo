package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vertigo/internal/api"
	"vertigo/internal/config"
	"vertigo/internal/proxy"

	"github.com/sirupsen/logrus"
)

// Server wraps the http.Server to provide graceful shutdown.
type Server struct {
	httpServer *http.Server
	log        *logrus.Logger
}

// New creates a new Server instance.
func New(cfg *config.Config, proxyManager *proxy.Manager, log *logrus.Logger) *Server {
	mux := http.NewServeMux()

	openAIAPI := api.NewOpenAIAPI(proxyManager, log)

	// Register handlers
	mux.HandleFunc("/openai/v1/chat/completions", openAIAPI.ChatCompletionsHandler)
	mux.HandleFunc("/openai/v1/models", openAIAPI.ModelsHandler)
	mux.HandleFunc("/openai/v1/models/", openAIAPI.ModelsHandler)

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
			Handler: mux,
		},
		log: log,
	}
}

// Run starts the server and waits for a shutdown signal.
func (s *Server) Run() {
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Fatalf("Could not listen on %s: %v\n", s.httpServer.Addr, err)
		}
	}()
	s.log.Infof("Server is ready to handle requests at %s", s.httpServer.Addr)

	// Wait for a shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() {
	s.log.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.log.Fatalf("Server shutdown failed: %v", err)
	}

	s.log.Info("Server gracefully stopped")
}

