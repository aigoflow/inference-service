package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aigoflow/inference-service/internal/llama"
	"github.com/aigoflow/inference-service/internal/models"
	"github.com/aigoflow/inference-service/internal/repository"
)

type InferenceRequest struct {
	TraceID string                 `json:"trace_id,omitempty"`
	ReqID   string                 `json:"req_id"`
	Input   string                 `json:"input"`
	Params  map[string]interface{} `json:"params"`
	ReplyTo string                 `json:"reply_to,omitempty"`
	Raw     bool                   `json:"raw,omitempty"`     // Bypass all formatting
}

type InferenceResponse struct {
	ReqID        string `json:"req_id"`
	Text         string `json:"text"`
	TokensIn     int    `json:"tokens_in"`
	TokensOut    int    `json:"tokens_out"`
	FinishReason string `json:"finish_reason"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

type InferenceService struct {
	llm            *llama.Model
	repo           repository.Repository
	grammarService *GrammarService
}

func NewInferenceService(llm *llama.Model, repo repository.Repository, grammarService *GrammarService) *InferenceService {
	return &InferenceService{
		llm:            llm,
		repo:           repo,
		grammarService: grammarService,
	}
}

func (s *InferenceService) ProcessInference(ctx context.Context, req InferenceRequest, source string, replyTo string, workerID string) (response *InferenceResponse, err error) {
	start := time.Now()
	
	// Add service-level crash recovery
	defer func() {
		if r := recover(); r != nil {
			duration := time.Since(start)
			errStr := fmt.Sprintf("service panic: %v", r)
			
			// Generate trace ID if not provided
			traceID := req.TraceID
			if traceID == "" {
				traceID = req.ReqID
			}
			
			// Log the panic using proper repository interface
			panicLog := &models.RequestLog{
				Timestamp:      start,
				TraceID:        traceID,
				ReqID:          req.ReqID,
				WorkerID:       workerID,
				Source:         source,
				ReplyTo:        replyTo,
				RawInput:       req.Input,
				FormattedInput: "[CRASHED]",
				ResponseText:   "[CRASHED]",
				ParamsJSON:     toJSON(req.Params),
				GrammarUsed:    "none",
				DurationMs:     duration.Milliseconds(),
				Status:         "panic",
				Error:          errStr,
			}
			s.repo.Request().LogRequest(ctx, panicLog)
			
			response = &InferenceResponse{
				ReqID:        req.ReqID,
				Text:         "",
				TokensIn:     0,
				TokensOut:    0,
				FinishReason: "error",
				DurationMs:   duration.Milliseconds(),
				Error:        errStr,
			}
			err = fmt.Errorf("service panic: %v", r)
		}
	}()
	
	// Generate trace ID if not provided
	traceID := req.TraceID
	if traceID == "" {
		traceID = req.ReqID // fallback to request ID
	}
	
	// Resolve grammar if provided
	grammarRef := getStringParam(req.Params, "grammar", "")
	resolvedGrammar := ""
	if grammarRef != "" {
		var grammarErr error
		resolvedGrammar, grammarErr = s.grammarService.ResolveGrammar(grammarRef)
		if grammarErr != nil {
			slog.Warn("Grammar resolution failed", "ref", grammarRef, "error", grammarErr)
			// Continue without grammar
		} else {
			// TEMPORARY: Disable grammar due to llama.cpp crashes
			slog.Warn("Grammar resolved but disabled due to known llama.cpp issues", "ref", grammarRef, "grammar_length", len(resolvedGrammar))
			// TODO: Enable when llama.cpp grammar bugs are fixed
			// updatedParams := make(map[string]interface{})
			// for k, v := range req.Params {
			// 	updatedParams[k] = v
			// }
			// updatedParams["grammar"] = resolvedGrammar
			// req.Params = updatedParams
		}
	}

	// Generate inference - use raw mode if requested
	var text string
	var tokensIn, tokensOut int
	var formattedInput string
	
	if req.Raw {
		// Raw mode: pass input directly to model without any formatting
		slog.Debug("Using raw mode - bypassing all formatting", "req_id", req.ReqID)
		text, tokensIn, tokensOut, formattedInput, err = s.llm.GenerateRaw(req.Input, req.Params)
	} else {
		// Normal mode: use formatting system
		text, tokensIn, tokensOut, formattedInput, err = s.llm.GenerateWithFormatting(req.Input, req.Params)
	}
	
	duration := time.Since(start)
	status := "ok"
	errStr := ""
	if err != nil {
		status = "error"
		errStr = err.Error()
		text = "" // Clear text on error
	}
	
	// Store request log using proper repository interface
	requestLog := &models.RequestLog{
		Timestamp:      start,
		TraceID:        traceID,
		ReqID:          req.ReqID,
		WorkerID:       workerID,
		Source:         source,
		ReplyTo:        replyTo,
		RawInput:       req.Input,
		FormattedInput: formattedInput,
		ResponseText:   text,
		InputLen:       len(req.Input),
		ParamsJSON:     toJSON(req.Params),
		GrammarUsed:    grammarRef,
		TokensIn:       tokensIn,
		TokensOut:      tokensOut,
		DurationMs:     duration.Milliseconds(),
		Status:         status,
		Error:          errStr,
	}
	
	if grammarRef == "" {
		requestLog.GrammarUsed = "none"
	}
	
	s.repo.Request().LogRequest(ctx, requestLog)
	
	response = &InferenceResponse{
		ReqID:        req.ReqID,
		Text:         text,
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		FinishReason: "stop",
		DurationMs:   duration.Milliseconds(),
	}
	
	if err != nil {
		response.Error = errStr
	}
	
	return response, err
}

func toJSON(v interface{}) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func getStringParam(params map[string]interface{}, key string, defaultVal string) string {
	if v, ok := params[key]; ok {
		if val, ok := v.(string); ok {
			return val
		}
	}
	return defaultVal
}

// GetRequestLogs retrieves request logs through proper repository interface
func (s *InferenceService) GetRequestLogs(ctx context.Context, limit int) ([]*models.RequestLog, error) {
	return s.repo.Request().GetRequestLogs(ctx, limit)
}

// GetRepository returns the repository for use by other services
func (s *InferenceService) GetRepository() repository.Repository {
	return s.repo
}