package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aigoflow/inference-service/internal/handlers"
	"github.com/aigoflow/inference-service/internal/llama"
	"github.com/aigoflow/inference-service/internal/services"
)

type Server struct {
	httpAddr         string
	inferenceService *services.InferenceService
	embeddingService *services.EmbeddingService
	grammarService   *services.GrammarService
	llm              *llama.Model
}

func NewServer(httpAddr string, inferenceService *services.InferenceService, grammarService *services.GrammarService, llm *llama.Model) *Server {
	return &Server{
		httpAddr:         httpAddr,
		inferenceService: inferenceService,
		grammarService:   grammarService,
		llm:              llm,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Always register inference endpoints (includes health, logs, and completions)
	inferenceHandler := handlers.NewInferenceHandler(s.inferenceService)
	inferenceHandler.RegisterRoutes(mux)
	
	// Additionally register embedding endpoints if model supports embeddings
	if s.llm.IsEmbeddingModel() {
		slog.Info("Model supports embeddings, additionally registering embedding endpoints")
		s.embeddingService = services.NewEmbeddingService(s.llm, s.inferenceService.GetRepository())
		embeddingHandler := handlers.NewEmbeddingHandler(s.embeddingService)
		embeddingHandler.RegisterRoutes(mux)
	}
	
	// Register grammar CRUD endpoints (available for both types)
	grammarHandler := handlers.NewGrammarHandler(s.grammarService)
	grammarHandler.RegisterRoutes(mux)
	
	slog.Info("HTTP server starting", "addr", s.httpAddr)
	
	return http.ListenAndServe(s.httpAddr, mux)
}