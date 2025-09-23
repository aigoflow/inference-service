package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aigoflow/inference-service/internal/services"
)

type InferenceHandler struct {
	inferenceService *services.InferenceService
}

func NewInferenceHandler(inferenceService *services.InferenceService) *InferenceHandler {
	return &InferenceHandler{
		inferenceService: inferenceService,
	}
}

func (h *InferenceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/completions", h.handleCompletions)
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/logs", h.handleLogs)
}

func (h *InferenceHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *InferenceHandler) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	
	var httpReq services.InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&httpReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	if httpReq.ReqID == "" {
		httpReq.ReqID = fmt.Sprintf("http-%d", time.Now().UnixNano())
	}
	
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		httpReq.TraceID = traceID
	}
	
	response, err := h.inferenceService.ProcessInference(r.Context(), httpReq, "http.inference", "direct", "http-worker")
	
	resp := map[string]interface{}{
		"req_id":     response.ReqID,
		"text":       response.Text,
		"tokens_in":  response.TokensIn,
		"tokens_out": response.TokensOut,
		"ms":         response.DurationMs,
	}
	if err != nil {
		resp["error"] = response.Error
	}
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *InferenceHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	
	logs, err := h.inferenceService.GetRequestLogs(r.Context(), limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}