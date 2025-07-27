package main

import (
	"flag"
	"log"

	"vertigo/internal/config"
	"vertigo/internal/db"
	"vertigo/internal/proxy"
	"vertigo/internal/server"
	"vertigo/internal/store"

	"github.com/sirupsen/logrus"
)

func main() {
	// --- Configuration ---
	configPath := flag.String("config", "vertigo.yaml", "path to the configuration file")
	flag.Parse()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	if len(cfg.Gemini.APIKeys) == 0 {
		logger.Fatal("No API keys found in the configuration")
	}

	// --- Database Initialization ---
	database, err := db.InitDB("vertigo.db")
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.CloseDB(database)

	// --- Dependencies ---
	keyManager := proxy.NewKeyManager(cfg.Gemini.APIKeys)
	convStore := store.NewConversationStore(database)
	proxyManager := proxy.NewManager(keyManager, convStore, logger)

	// --- HTTP Server ---
	srv := server.New(cfg, proxyManager, logger)

	log.Printf("Server starting on %s:%d", cfg.Server.Host, cfg.Server.Port)
	srv.Run()
}
