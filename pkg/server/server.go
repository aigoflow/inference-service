package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aigoflow/inference-service/internal/handlers"
	"github.com/aigoflow/inference-service/internal/llama"
	"github.com/aigoflow/inference-service/internal/services"
	"github.com/aigoflow/inference-service/internal/capabilities"
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
	
	// Detect model capabilities dynamically
	detector := capabilities.NewAutoCapabilityDetector()
	detectedCapabilities := detector.DetectCapabilities(s.llm)
	
	slog.Info("Starting server with detected capabilities",
		"model", s.llm.GetModelMetadata().Additional["config_name"],
		"architecture", s.llm.GetModelArchitecture(),
		"capabilities", detector.GetCapabilitiesSummary(detectedCapabilities))
	
	// Register endpoints based on detected capabilities
	endpointsRegistered := 0
	
	for _, cap := range detectedCapabilities {
		switch cap.Type {
		case capabilities.CapabilityTextGeneration:
			inferenceHandler := handlers.NewInferenceHandler(s.inferenceService)
			inferenceHandler.RegisterRoutes(mux)
			slog.Info("Registered text generation endpoints", "endpoints", []string{"/v1/completions", "/healthz", "/logs"})
			endpointsRegistered++
			
		case capabilities.CapabilityEmbeddings:
			s.embeddingService = services.NewEmbeddingService(s.llm, s.inferenceService.GetRepository())
			embeddingHandler := handlers.NewEmbeddingHandler(s.embeddingService)
			embeddingHandler.RegisterRoutes(mux)
			if dim, ok := cap.Parameters["dimension"].(int); ok {
				slog.Info("Registered embedding endpoints", "endpoints", []string{"/v1/embeddings"}, "dimension", dim)
			} else {
				slog.Info("Registered embedding endpoints", "endpoints", []string{"/v1/embeddings"})
			}
			endpointsRegistered++
			
		case capabilities.CapabilityGrammarConstrained:
			grammarHandler := handlers.NewGrammarHandler(s.grammarService)
			grammarHandler.RegisterRoutes(mux)
			slog.Info("Registered grammar endpoints", "endpoints", []string{"/grammars"})
			endpointsRegistered++
			
		case capabilities.CapabilityImageUnderstanding:
			// Future: Register image processing endpoints
			slog.Info("Image understanding capability detected (endpoints not implemented yet)")
			
		case capabilities.CapabilityAudioTranscription:
			// Future: Register audio processing endpoints
			slog.Info("Audio transcription capability detected (endpoints not implemented yet)")
			
		case capabilities.CapabilityToolCalling:
			// Future: Register tool calling endpoints
			slog.Info("Tool calling capability detected (endpoints not implemented yet)")
		}
	}
	
	// Ensure we always have basic endpoints registered
	if endpointsRegistered == 0 {
		slog.Warn("No capabilities detected, registering fallback endpoints")
		inferenceHandler := handlers.NewInferenceHandler(s.inferenceService)
		inferenceHandler.RegisterRoutes(mux)
		grammarHandler := handlers.NewGrammarHandler(s.grammarService)
		grammarHandler.RegisterRoutes(mux)
	}
	
	slog.Info("HTTP server starting", 
		"addr", s.httpAddr,
		"endpoints_registered", endpointsRegistered,
		"total_capabilities", len(detectedCapabilities))
	
	return http.ListenAndServe(s.httpAddr, mux)
}