package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aigoflow/inference-service/internal/handlers"
	"github.com/aigoflow/inference-service/internal/llama"
	"github.com/aigoflow/inference-service/internal/services"
	"github.com/aigoflow/inference-service/internal/capabilities"
	"github.com/aigoflow/inference-service/internal/repository"
)

type Server struct {
	httpAddr         string
	inferenceService *services.InferenceService
	embeddingService *services.EmbeddingService
	audioService     *services.AudioService
	grammarService   *services.GrammarService
	repo             repository.Repository
	llm              interface{}
}

func NewServer(httpAddr string, inferenceService *services.InferenceService, grammarService *services.GrammarService, llm interface{}, repo repository.Repository) *Server {
	return &Server{
		httpAddr:         httpAddr,
		inferenceService: inferenceService,
		grammarService:   grammarService,
		llm:              llm,
		repo:             repo,
	}
}


func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Detect model capabilities dynamically
	detector := capabilities.NewAutoCapabilityDetector()
	var detectedCapabilities []capabilities.Capability
	
	// Check if model implements ModelInterface for capability detection
	if modelInterface, ok := s.llm.(capabilities.ModelInterface); ok {
		detectedCapabilities = detector.DetectCapabilities(modelInterface)
	} else {
		// Fallback: detect based on type
		if whisperModel, ok := s.llm.(services.WhisperInterface); ok && whisperModel.IsAudioModel() {
			detectedCapabilities = append(detectedCapabilities, capabilities.Capability{
				Type:        capabilities.CapabilityAudioTranscription,
				Version:     "1.0",
				Description: "Transcribe audio to text",
			})
		}
	}
	
	// Get model info depending on type
	var modelName, architecture string
	if llamaModel, ok := s.llm.(*llama.Model); ok {
		if configName, exists := llamaModel.GetModelMetadata().Additional["config_name"]; exists {
			if name, ok := configName.(string); ok {
				modelName = name
			} else {
				modelName = "llama-model"
			}
		} else {
			modelName = "llama-model"
		}
		architecture = llamaModel.GetModelArchitecture()
	} else if whisperModel, ok := s.llm.(services.WhisperInterface); ok {
		modelName = whisperModel.GetModelName()
		architecture = "whisper"
	} else {
		modelName = "unknown"
		architecture = "unknown"
	}
	
	slog.Info("Starting server with detected capabilities",
		"model", modelName,
		"architecture", architecture,
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
			if llamaModel, ok := s.llm.(*llama.Model); ok {
				s.embeddingService = services.NewEmbeddingService(llamaModel, s.inferenceService.GetRepository())
				embeddingHandler := handlers.NewEmbeddingHandler(s.embeddingService)
				embeddingHandler.RegisterRoutes(mux)
				if dim, ok := cap.Parameters["dimension"].(int); ok {
					slog.Info("Registered embedding endpoints", "endpoints", []string{"/v1/embeddings"}, "dimension", dim)
				} else {
					slog.Info("Registered embedding endpoints", "endpoints", []string{"/v1/embeddings"})
				}
				endpointsRegistered++
			} else {
				slog.Info("Embedding capability detected but no compatible model available")
			}
			
		case capabilities.CapabilityGrammarConstrained:
			grammarHandler := handlers.NewGrammarHandler(s.grammarService)
			grammarHandler.RegisterRoutes(mux)
			slog.Info("Registered grammar endpoints", "endpoints", []string{"/grammars"})
			endpointsRegistered++
			
		case capabilities.CapabilityImageUnderstanding:
			// Future: Register image processing endpoints
			slog.Info("Image understanding capability detected (endpoints not implemented yet)")
			
		case capabilities.CapabilityAudioTranscription:
			// Use audio service if available
			if s.audioService != nil {
				audioHandler := handlers.NewAudioHandler(s.audioService)
				audioHandler.RegisterRoutes(mux)
				slog.Info("Registered audio transcription endpoints", "endpoints", []string{"/v1/audio/transcriptions"})
				endpointsRegistered++
			} else if whisperModel, ok := s.llm.(services.WhisperInterface); ok {
				// Create audio service dynamically if not set
				s.audioService = services.NewAudioService(whisperModel, s.repo)
				audioHandler := handlers.NewAudioHandler(s.audioService)
				audioHandler.RegisterRoutes(mux)
				slog.Info("Registered audio transcription endpoints", "endpoints", []string{"/v1/audio/transcriptions"})
				endpointsRegistered++
			} else {
				slog.Info("Audio transcription capability detected but no audio model available")
			}
			
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

func (s *Server) SetAudioService(audioService *services.AudioService) {
	s.audioService = audioService
}