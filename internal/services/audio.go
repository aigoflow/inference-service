package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/aigoflow/inference-service/internal/repository"
	"github.com/aigoflow/inference-service/internal/types"
)

// AudioRequest represents a request for audio transcription
type AudioRequest struct {
	TraceID       string `json:"trace_id,omitempty"`
	ReqID         string `json:"req_id"`
	AudioURL      string `json:"audio_url,omitempty"`      // URL to audio file (preferred for NATS)
	AudioBase64   string `json:"audio_base64,omitempty"`   // Base64 encoded audio (small files only)
	Audio         []byte `json:"audio,omitempty"`          // Raw audio data (HTTP multipart)
	Model         string `json:"model,omitempty"`          // Whisper model to use
	Language      string `json:"language,omitempty"`       // Language hint  
	ReplyTo       string `json:"reply_to,omitempty"`       // NATS reply subject
	StreamChunks  bool   `json:"stream_chunks,omitempty"`  // For future real-time streaming
}

// AudioResponse represents the response from audio transcription
type AudioResponse struct {
	ReqID      string `json:"req_id"`
	Text       string `json:"text"`
	Language   string `json:"language"`
	Segments   []types.AudioSegment `json:"segments,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// AudioSegment represents a transcription segment with timing
type AudioSegment struct {
	ID     int     `json:"id"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Text   string  `json:"text"`
}

// AudioService handles audio transcription operations
type AudioService struct {
	whisper WhisperInterface
	repo    repository.Repository
}

// WhisperInterface defines the interface for Whisper model operations
type WhisperInterface interface {
	Transcribe(audioData []byte, language string) (string, []types.AudioSegment, error)
	GetModelName() string
	GetLanguages() []string
	IsAudioModel() bool
}

func NewAudioService(whisper WhisperInterface, repo repository.Repository) *AudioService {
	return &AudioService{
		whisper: whisper,
		repo:    repo,
	}
}

func (s *AudioService) ProcessTranscription(ctx context.Context, req AudioRequest, source string, replyTo string, workerID string) (*AudioResponse, error) {
	start := time.Now()
	
	// Generate trace ID if not provided
	traceID := req.TraceID
	if traceID == "" {
		traceID = req.ReqID
	}
	
	slog.Debug("Processing audio transcription request",
		"worker_id", workerID,
		"req_id", req.ReqID,
		"trace_id", traceID,
		"source", source,
		"model", req.Model)
	
	// Validate input - check all possible audio sources
	var audioData []byte
	var err error
	
	if len(req.Audio) > 0 {
		// Direct audio data (HTTP multipart)
		audioData = req.Audio
	} else if req.AudioBase64 != "" {
		// Base64 encoded audio (NATS small files)
		audioData, err = base64.StdEncoding.DecodeString(req.AudioBase64)
		if err != nil {
			return &AudioResponse{
				ReqID: req.ReqID,
				Error: "Invalid base64 audio data",
			}, fmt.Errorf("base64 decode failed: %w", err)
		}
	} else if req.AudioURL != "" {
		// Audio URL (future: download and process)
		return &AudioResponse{
			ReqID: req.ReqID,
			Error: "Audio URL processing not implemented yet",
		}, fmt.Errorf("audio URL processing not implemented")
	} else {
		return &AudioResponse{
			ReqID: req.ReqID,
			Error: "No audio data, base64, or URL provided",
		}, fmt.Errorf("no audio input provided")
	}
	
	// Perform transcription
	text, segments, err := s.whisper.Transcribe(audioData, req.Language)
	if err != nil {
		errorStr := fmt.Sprintf("Transcription failed: %v", err)
		return &AudioResponse{
			ReqID: req.ReqID,
			Error: errorStr,
		}, err
	}
	
	duration := time.Since(start)
	
	// Create response
	response := &AudioResponse{
		ReqID:      req.ReqID,
		Text:       text,
		Language:   req.Language,
		Segments:   segments,
		DurationMs: duration.Milliseconds(),
	}
	
	// Log successful transcription
	slog.Info("Audio transcription completed",
		"worker_id", workerID,
		"req_id", req.ReqID,
		"duration_ms", duration.Milliseconds(),
		"text_length", len(text),
		"segments", len(segments))
	
	// Store transcription in database for auditing
	if s.repo != nil {
		// This would use the repository pattern to store transcription logs
		// Similar to how inference requests are stored
	}
	
	return response, nil
}

func (s *AudioService) GetRepository() repository.Repository {
	return s.repo
}