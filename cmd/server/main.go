package main

import (
	"flag"
	"net/http"

	"vertigo/internal/config"
	"vertigo/internal/handler"
	"vertigo/internal/middleware"
	"vertigo/internal/proxy"
	"vertigo/internal/server"

	"github.com/sirupsen/logrus"
)

func main() {
	// --- Configuration ---
	configPath := flag.String("config", "config.yaml", "path to the configuration file")
	flag.Parse()

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if len(cfg.Gemini.APIKeys) == 0 {
		log.Fatal("No API keys found in the configuration")
	}

	// --- Dependencies ---
	keyRotator := proxy.NewKeyRotator(cfg.Gemini.APIKeys)

	// --- HTTP Server ---
	proxyHandler := handler.NewProxyHandler(keyRotator, log)
	loggedProxyHandler := middleware.Logger(proxyHandler, log)

	mux := http.NewServeMux()
	mux.Handle("/openai/v1/chat/completions", loggedProxyHandler)

	srv := server.New(cfg.Server.Port, cfg.Server.Host, mux, log)

	srv.Run()
}
