package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/aigoflow/inference-service/internal/config"
	"github.com/aigoflow/inference-service/internal/llama"
	"github.com/aigoflow/inference-service/internal/repository"
	"github.com/aigoflow/inference-service/internal/services"
	"github.com/aigoflow/inference-service/internal/store"
	"github.com/aigoflow/inference-service/pkg/server"
)

func main() {
	var envFile = flag.String("env", "", "Optional .env file to load")
	flag.Parse()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load(*envFile)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize database
	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	
	// Log startup event
	db.Event("info", "startup", "Server starting", map[string]interface{}{
		"model_name": cfg.ModelName,
		"http_addr":  cfg.HTTPAddr,
		"db_path":    cfg.DBPath,
	})

	// Initialize repository  
	grammarPath := filepath.Join(cfg.DataDir, "grammars")
	repo := repository.NewSQLiteRepository(db, grammarPath)

	// Log model loading start
	db.Event("info", "model.loading", "Model loading started", map[string]interface{}{
		"model_path": cfg.ModelPath,
		"model_name": cfg.ModelName,
		"threads":    cfg.Threads,
		"ctx_size":   cfg.CtxSize,
	})

	// Load model with system configuration (with auto-download if missing)
	llm, err := llama.LoadWithAutoDownload(cfg.ModelPath, cfg.ModelURL, llama.Config{
		ModelPath: cfg.ModelPath,
		ModelName: cfg.ModelName,
		Threads:   cfg.Threads,
		CtxSize:   cfg.CtxSize,
	}, cfg)
	if err != nil {
		// Log model loading failure
		db.Event("error", "model.failed", "Model loading failed", map[string]interface{}{
			"model_path": cfg.ModelPath,
			"error":      err.Error(),
		})
		slog.Error("Failed to load model", "error", err)
		os.Exit(1)
	}
	
	// Log model loading success
	db.Event("info", "model.loaded", "Model loaded successfully", map[string]interface{}{
		"model_path": cfg.ModelPath,
		"model_name": cfg.ModelName,
	})

	// Initialize services
	grammarService := services.NewGrammarService(grammarPath)
	inferenceService := services.NewInferenceService(llm, repo, grammarService)

	// Log services initialization
	db.Event("info", "services.init", "Initializing services", map[string]interface{}{
		"http_addr": cfg.HTTPAddr,
		"nats_url":  cfg.NatsURL,
	})

	// Initialize NATS service
	natsService, err := services.NewNATSService(cfg, inferenceService)
	if err != nil {
		db.Event("error", "nats.failed", "NATS service initialization failed", map[string]interface{}{
			"nats_url": cfg.NatsURL,
			"error":    err.Error(),
		})
		slog.Error("Failed to create NATS service", "error", err)
		os.Exit(1)
	}
	
	// Initialize Health service for model discovery with capability detection
	healthService := services.NewHealthService(natsService.GetConnection(), cfg, llm, natsService.GetMonitoringService())

	// Start HTTP server
	httpServer := server.NewServer(cfg.HTTPAddr, inferenceService, grammarService, llm)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Log server ready
	db.Event("info", "server.ready", "Server ready to accept requests", map[string]interface{}{
		"http_addr":  cfg.HTTPAddr,
		"model_name": cfg.ModelName,
		"nats_url":   cfg.NatsURL,
	})

	// Start all services
	go func() {
		if err := httpServer.Start(ctx); err != nil {
			db.Event("error", "http.failed", "HTTP server failed", map[string]interface{}{
				"error": err.Error(),
			})
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	go func() {
		if err := natsService.Start(ctx); err != nil {
			db.Event("error", "nats.failed", "NATS service failed", map[string]interface{}{
				"error": err.Error(),
			})
			slog.Error("NATS service failed", "error", err)
		}
	}()
	
	go func() {
		if err := healthService.Start(ctx); err != nil {
			db.Event("error", "health.failed", "Health service failed", map[string]interface{}{
				"error": err.Error(),
			})
			slog.Error("Health service failed", "error", err)
		}
	}()

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	
	slog.Info("Shutting down server")
}