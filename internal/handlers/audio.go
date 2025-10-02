package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/aigoflow/inference-service/internal/services"
)

type AudioHandler struct {
	audioService *services.AudioService
}

func NewAudioHandler(audioService *services.AudioService) *AudioHandler {
	return &AudioHandler{
		audioService: audioService,
	}
}

func (h *AudioHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/audio/transcriptions", h.handleTranscriptions)
	mux.HandleFunc("/transcribe", h.handleTranscriptions) // Legacy endpoint
}

func (h *AudioHandler) handleTranscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB limit
		http.Error(w, fmt.Sprintf("Failed to parse multipart form: %v", err), http.StatusBadRequest)
		return
	}
	
	// Get the uploaded file
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded or invalid file field", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Read file data
	audioData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusBadRequest)
		return
	}
	
	// Get optional parameters
	model := r.FormValue("model")
	if model == "" {
		model = "whisper-1" // Default model name
	}
	
	language := r.FormValue("language")
	
	// Create request
	httpReq := services.AudioRequest{
		ReqID:    fmt.Sprintf("audio-http-%d", time.Now().UnixNano()),
		Audio:    audioData,
		Model:    model,
		Language: language,
	}
	
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		httpReq.TraceID = traceID
	}
	
	// Process transcription
	slog.Info("Starting HTTP audio transcription", "req_id", httpReq.ReqID, "audio_size", len(audioData))
	response, err := h.audioService.ProcessTranscription(r.Context(), httpReq, "http.transcription", "direct", "http-worker")
	slog.Info("Completed HTTP audio transcription", "req_id", httpReq.ReqID, "response_text_len", len(response.Text), "error", err)
	
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": response.Error,
				"type":    "transcription_error",
			},
		})
		return
	}
	
	// Return OpenAI-compatible response format
	openAIResponse := map[string]interface{}{
		"text": response.Text,
	}
	
	// Add segments if requested
	if r.FormValue("response_format") == "verbose_json" {
		segments := make([]map[string]interface{}, len(response.Segments))
		for i, seg := range response.Segments {
			segments[i] = map[string]interface{}{
				"id":               seg.ID,
				"seek":             0,
				"start":            seg.Start,
				"end":              seg.End,
				"text":             seg.Text,
				"tokens":           []int{},
				"temperature":      0.0,
				"avg_logprob":      -1.0,
				"compression_ratio": 0.0,
				"no_speech_prob":   0.0,
			}
		}
		openAIResponse = map[string]interface{}{
			"task":     "transcribe",
			"language": response.Language,
			"duration": float64(response.DurationMs) / 1000.0,
			"text":     response.Text,
			"segments": segments,
		}
	}
	
	_ = json.NewEncoder(w).Encode(openAIResponse)
}