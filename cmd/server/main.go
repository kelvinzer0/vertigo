package main

import (
	"flag"
	"net/http"

	"vertigo/internal/config"
	"vertigo/internal/handler"
	"vertigo/internal/middleware"
	"vertigo/internal/proxy"
	"vertigo/internal/server"
	"vertigo/internal/store"

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
	convStore := store.NewConversationStore()

	// --- HTTP Server ---
	proxyHandler := handler.NewProxyHandler(keyRotator, convStore, log)
	loggedProxyHandler := middleware.Logger(proxyHandler, log)

	embeddingHandler := handler.NewEmbeddingHandler(keyRotator, log)
	loggedEmbeddingHandler := middleware.Logger(embeddingHandler, log)

	completionsHandler := handler.NewCompletionsHandler(keyRotator, log)
	loggedCompletionsHandler := middleware.Logger(completionsHandler, log)

	mux := http.NewServeMux()
	mux.Handle("/openai/v1/chat/completions", loggedProxyHandler)
	mux.HandleFunc("/openai/v1/models", handler.ModelsHandler)
	mux.HandleFunc("/openai/v1/models/", handler.ModelsHandler)
	mux.Handle("/openai/v1/embeddings", loggedEmbeddingHandler)
	mux.Handle("/openai/v1/completions", loggedCompletionsHandler)

	srv := server.New(cfg.Server.Port, cfg.Server.Host, mux, log)

	srv.Run()
}
