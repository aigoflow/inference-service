package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aigoflow/inference-service/internal/handlers"
	"github.com/aigoflow/inference-service/internal/services"
)

type Server struct {
	httpAddr         string
	inferenceService *services.InferenceService
	grammarService   *services.GrammarService
}

func NewServer(httpAddr string, inferenceService *services.InferenceService, grammarService *services.GrammarService) *Server {
	return &Server{
		httpAddr:         httpAddr,
		inferenceService: inferenceService,
		grammarService:   grammarService,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Register inference endpoints
	inferenceHandler := handlers.NewInferenceHandler(s.inferenceService)
	inferenceHandler.RegisterRoutes(mux)
	
	// Register grammar CRUD endpoints
	grammarHandler := handlers.NewGrammarHandler(s.grammarService)
	grammarHandler.RegisterRoutes(mux)
	
	slog.Info("HTTP server starting", "addr", s.httpAddr)
	
	return http.ListenAndServe(s.httpAddr, mux)
}