package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aigoflow/inference-service/internal/llama"
	"github.com/aigoflow/inference-service/internal/models"
	"github.com/aigoflow/inference-service/internal/repository"
)

type EmbeddingRequest struct {
	TraceID string                 `json:"trace_id,omitempty"`
	ReqID   string                 `json:"req_id"`
	Input   interface{}            `json:"input"`    // Can be string or []string
	Model   string                 `json:"model,omitempty"`
	ReplyTo string                 `json:"reply_to,omitempty"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
	Error  string          `json:"error,omitempty"`
}

type EmbeddingService struct {
	llm  *llama.Model
	repo repository.Repository
}

func NewEmbeddingService(llm *llama.Model, repo repository.Repository) *EmbeddingService {
	return &EmbeddingService{
		llm:  llm,
		repo: repo,
	}
}

func (s *EmbeddingService) ProcessEmbedding(ctx context.Context, req EmbeddingRequest, source string, replyTo string, workerID string) (response *EmbeddingResponse, err error) {
	start := time.Now()
	
	// Validate that model supports embeddings
	if !s.llm.IsEmbeddingModel() {
		return &EmbeddingResponse{
			Error: "Model does not support embedding generation",
		}, fmt.Errorf("model does not support embeddings")
	}
	
	// Convert input to string array
	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []string:
		inputs = v
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				inputs = append(inputs, str)
			} else {
				return &EmbeddingResponse{
					Error: "All input items must be strings",
				}, fmt.Errorf("invalid input type in array")
			}
		}
	default:
		return &EmbeddingResponse{
			Error: "Input must be a string or array of strings",
		}, fmt.Errorf("invalid input type")
	}
	
	if len(inputs) == 0 {
		return &EmbeddingResponse{
			Error: "No input provided",
		}, fmt.Errorf("empty input")
	}
	
	// Process each input
	var embeddingData []EmbeddingData
	totalTokens := 0
	
	for i, input := range inputs {
		embedding, tokensIn, err := s.llm.GenerateEmbedding(input)
		if err != nil {
			slog.Error("Embedding generation failed", "error", err, "input_index", i)
			return &EmbeddingResponse{
				Error: fmt.Sprintf("Embedding generation failed: %v", err),
			}, err
		}
		
		totalTokens += tokensIn
		
		embeddingData = append(embeddingData, EmbeddingData{
			Object:    "embedding",
			Embedding: embedding,
			Index:     i,
		})
	}
	
	duration := time.Since(start)
	
	response = &EmbeddingResponse{
		Object: "list",
		Data:   embeddingData,
		Model:  req.Model,
		Usage: EmbeddingUsage{
			PromptTokens: totalTokens,
			TotalTokens:  totalTokens,
		},
	}
	
	// Log the request
	status := "ok"
	if err != nil {
		status = "error"
	}
	
	logData := models.RequestLog{
		ReqID:          req.ReqID,
		TraceID:        req.TraceID,
		Source:         source,
		WorkerID:       workerID,
		RawInput:       fmt.Sprintf("%v", req.Input),
		FormattedInput: fmt.Sprintf("%v", req.Input), // No formatting for embeddings
		TokensIn:       totalTokens,
		TokensOut:      0, // Embeddings don't generate tokens
		DurationMs:     duration.Milliseconds(),
		Status:         status,
		ReplyTo:        replyTo,
		EmbeddingSize:  s.llm.GetEmbeddingSize(),
		EmbeddingCount: len(embeddingData),
	}
	
	if logErr := s.repo.Request().LogRequest(ctx, &logData); logErr != nil {
		slog.Error("Failed to log embedding request", "error", logErr)
	}
	
	return response, nil
}

func (s *EmbeddingService) GetRequestLogs(ctx context.Context, limit int) ([]*models.RequestLog, error) {
	return s.repo.Request().GetRequestLogs(ctx, limit)
}