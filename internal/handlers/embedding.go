package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aigoflow/inference-service/internal/services"
)

type EmbeddingHandler struct {
	embeddingService *services.EmbeddingService
}

func NewEmbeddingHandler(embeddingService *services.EmbeddingService) *EmbeddingHandler {
	return &EmbeddingHandler{
		embeddingService: embeddingService,
	}
}

func (h *EmbeddingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/embeddings", h.handleEmbeddings)
	mux.HandleFunc("/embedding", h.handleEmbeddings) // Legacy endpoint
}

func (h *EmbeddingHandler) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	
	var httpReq services.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&httpReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	if httpReq.ReqID == "" {
		httpReq.ReqID = fmt.Sprintf("emb-http-%d", time.Now().UnixNano())
	}
	
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		httpReq.TraceID = traceID
	}
	
	response, err := h.embeddingService.ProcessEmbedding(r.Context(), httpReq, "http.embedding", "direct", "http-worker")
	
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": response.Error,
				"type":    "embedding_error",
			},
		})
		return
	}
	
	_ = json.NewEncoder(w).Encode(response)
}