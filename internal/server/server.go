package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Server wraps the http.Server to provide graceful shutdown.
type Server struct {
	httpServer *http.Server
	log        *logrus.Logger
}

// New creates a new Server instance.
func New(port int, host string, handler http.Handler, log *logrus.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", host, port),
			Handler: handler,
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
