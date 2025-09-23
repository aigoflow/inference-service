package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/aigoflow/inference-service/internal/models"
	"github.com/aigoflow/inference-service/internal/services"
)

type GrammarHandler struct {
	grammarService *services.GrammarService
}

func NewGrammarHandler(grammarService *services.GrammarService) *GrammarHandler {
	return &GrammarHandler{
		grammarService: grammarService,
	}
}

// RegisterRoutes registers all grammar-related routes
func (h *GrammarHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/grammars", h.handleGrammars)
	mux.HandleFunc("/grammars/", h.handleGrammarPath)
}

func (h *GrammarHandler) handleGrammars(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listDirectories(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *GrammarHandler) handleGrammarPath(w http.ResponseWriter, r *http.Request) {
	// Parse path: /grammars/{dir} or /grammars/{dir}/{name}
	path := strings.TrimPrefix(r.URL.Path, "/grammars/")
	parts := strings.Split(path, "/")
	
	if len(parts) == 1 && parts[0] != "" {
		// /grammars/{dir}
		h.handleDirectory(w, r, parts[0])
	} else if len(parts) == 2 && parts[1] != "" {
		// /grammars/{dir}/{name}
		h.handleGrammar(w, r, parts[0], parts[1])
	} else {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
	}
}

func (h *GrammarHandler) handleDirectory(w http.ResponseWriter, r *http.Request, dir string) {
	switch r.Method {
	case http.MethodGet:
		h.listGrammars(w, r, dir)
	case http.MethodPost:
		h.createDirectory(w, r, dir)
	case http.MethodDelete:
		h.deleteDirectory(w, r, dir)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *GrammarHandler) handleGrammar(w http.ResponseWriter, r *http.Request, dir, name string) {
	switch r.Method {
	case http.MethodGet:
		h.getGrammar(w, r, dir, name)
	case http.MethodPost:
		h.createGrammar(w, r, dir, name)
	case http.MethodPut:
		h.updateGrammar(w, r, dir, name)
	case http.MethodDelete:
		h.deleteGrammar(w, r, dir, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Directory operations
func (h *GrammarHandler) listDirectories(w http.ResponseWriter, r *http.Request) {
	response, err := h.grammarService.ListDirectories()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list directories: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *GrammarHandler) listGrammars(w http.ResponseWriter, r *http.Request, dir string) {
	response, err := h.grammarService.ListGrammars(dir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list grammars: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *GrammarHandler) createDirectory(w http.ResponseWriter, r *http.Request, dir string) {
	req := models.CreateDirectoryRequest{Name: dir}
	
	if err := h.grammarService.CreateDirectory(req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create directory: %v", err), http.StatusBadRequest)
		return
	}
	
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Directory created", "name": dir})
}

func (h *GrammarHandler) deleteDirectory(w http.ResponseWriter, r *http.Request, dir string) {
	if err := h.grammarService.DeleteDirectory(dir); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete directory: %v", err), http.StatusBadRequest)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Directory deleted", "name": dir})
}

// Grammar operations
func (h *GrammarHandler) getGrammar(w http.ResponseWriter, r *http.Request, dir, name string) {
	grammar, err := h.grammarService.GetGrammar(dir, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get grammar: %v", err), http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grammar)
}

func (h *GrammarHandler) createGrammar(w http.ResponseWriter, r *http.Request, dir, name string) {
	var req models.CreateGrammarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Override name and directory from URL path
	req.Name = name
	req.Directory = dir
	
	grammar, err := h.grammarService.CreateGrammar(req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, fmt.Sprintf("Failed to create grammar: %v", err), http.StatusBadRequest)
		}
		return
	}
	
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grammar)
}

func (h *GrammarHandler) updateGrammar(w http.ResponseWriter, r *http.Request, dir, name string) {
	var req models.UpdateGrammarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	grammar, err := h.grammarService.UpdateGrammar(dir, name, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to update grammar: %v", err), http.StatusBadRequest)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grammar)
}

func (h *GrammarHandler) deleteGrammar(w http.ResponseWriter, r *http.Request, dir, name string) {
	if err := h.grammarService.DeleteGrammar(dir, name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to delete grammar: %v", err), http.StatusInternalServerError)
		}
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Grammar deleted", "name": name})
}