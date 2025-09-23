package llama

/*
#cgo CXXFLAGS: -I${SRCDIR}/include -I${SRCDIR}/src -I${SRCDIR}/ggml_include -std=c++11 -DGGML_USE_METAL
#cgo LDFLAGS: -L${SRCDIR} -lbinding -lllama -lggml -lggml-base -lggml-cpu -lggml-blas -lggml-metal -lm
#cgo darwin LDFLAGS: -framework Accelerate -framework Foundation -framework Metal -framework MetalKit -framework MetalPerformanceShaders
#cgo linux LDFLAGS: -lstdc++
#include "binding.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"
)

// Config holds the configuration for the llama model
type Config struct {
	ModelPath string
	ModelName string // Add model name for prompt formatting
	Threads   int
	CtxSize   int
}

type Model struct {
	model  unsafe.Pointer
	config Config
	// Remove ctx - we'll create fresh context for each request
}

func Load(cfg Config) (*Model, error) {
	modelPath := C.CString(cfg.ModelPath)
	defer C.free(unsafe.Pointer(modelPath))
	
	// Load model with GPU layers if supported
	gpuLayers := 0
	if hasGPUSupport() {
		gpuLayers = 99 // Use all layers on GPU if available
	}
	
	slog.Info("Loading model with CGO", 
		"path", cfg.ModelPath, 
		"threads", cfg.Threads, 
		"ctx_size", cfg.CtxSize,
		"gpu_layers", gpuLayers,
		"gpu_support", hasGPUSupport())
	
	model := C.load_model(modelPath, C.int(cfg.CtxSize), C.int(cfg.Threads), C.int(gpuLayers), C.bool(true), C.bool(false))
	if model == nil {
		return nil, fmt.Errorf("failed to load model: %s", cfg.ModelPath)
	}
	
	slog.Info("Model loaded successfully", "gpu_support", hasGPUSupport())
	
	// Set finalizer to clean up resources  
	m := &Model{
		model:  model,
		config: cfg,
	}
	runtime.SetFinalizer(m, (*Model).cleanup)
	
	return m, nil
}

func (m *Model) GenerateWithFormatting(input string, params map[string]interface{}) (text string, tokensIn, tokensOut int, formattedInput string, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Inference panic recovered", "error", r)
			text, tokensIn, tokensOut, formattedInput, err = "", 0, 0, "", fmt.Errorf("inference panic: %v", r)
		}
	}()
	
	if m.model == nil {
		return "", 0, 0, "", fmt.Errorf("model is nil")
	}
	
	// Create fresh context per request for stateless operation
	ctx := C.new_context(m.model, C.int(m.config.CtxSize), C.int(m.config.Threads))
	if ctx == nil {
		return "", 0, 0, "", fmt.Errorf("failed to create context")
	}
	defer C.free_context(ctx)
	
	// Apply model-specific prompt formatting
	formattedInput = formatPromptForModel(input, m.config.ModelPath)
	
	// Extract generation parameters
	maxTokens := getIntParam(params, "max_tokens", 512)
	temperature := getFloatParam(params, "temperature", 0.7)
	topP := getFloatParam(params, "top_p", 1.0)
	topK := getIntParam(params, "top_k", 40)
	repeatPenalty := getFloatParam(params, "repeat_penalty", 1.1)
	repeatLastN := getIntParam(params, "repeat_last_n", 64)
	
	// Natural stopping when max_tokens not specified
	if _, exists := params["max_tokens"]; !exists {
		maxTokens = 2048
	}
	
	// Count input tokens using formatted prompt
	inputCStr := C.CString(formattedInput)
	defer C.free(unsafe.Pointer(inputCStr))
	tokensIn = int(C.count_tokens(ctx, inputCStr))
	
	// Generate tokens using proven stable prediction
	resultSize := maxTokens * 4
	result := make([]byte, resultSize)
	
	tokensOut = int(C.llama_predict(
		ctx,
		inputCStr,
		(*C.char)(unsafe.Pointer(&result[0])),
		C.int(resultSize),
		C.int(maxTokens),
		C.float(temperature),
		C.float(topP),
		C.int(topK),
		C.float(repeatPenalty),
		C.int(repeatLastN),
		C.bool(true),
	))
	
	if tokensOut < 0 {
		return "", tokensIn, 0, formattedInput, fmt.Errorf("inference failed")
	}
	
	text = C.GoString((*C.char)(unsafe.Pointer(&result[0])))
	return text, tokensIn, tokensOut, formattedInput, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (m *Model) cleanup() {
	if m.model != nil {
		C.free_model(m.model)
		m.model = nil
	}
}

func hasGPUSupport() bool {
	return bool(C.has_gpu_support())
}

// LoadWithAutoDownload loads a model, downloading it if missing
func LoadWithAutoDownload(modelPath, modelURL string, cfg Config) (*Model, error) {
	// Check if model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		if modelURL == "" {
			return nil, fmt.Errorf("model not found at %s and no download URL provided", modelPath)
		}
		
		slog.Info("Model not found, downloading...", "url", modelURL, "path", modelPath)
		
		// Create directory
		if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create model directory: %w", err)
		}
		
		// Download model
		if err := downloadFile(modelURL, modelPath); err != nil {
			return nil, fmt.Errorf("failed to download model: %w", err)
		}
		
		slog.Info("Model downloaded successfully", "path", modelPath)
	}
	
	// Load the model normally
	return Load(cfg)
}

// downloadFile downloads a file from URL to local path with progress logging
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get file size
	size := resp.ContentLength
	sizeMB := float64(size) / 1024 / 1024

	if size > 0 {
		slog.Info("Starting download", 
			"size_mb", fmt.Sprintf("%.1f", sizeMB), 
			"size_gb", fmt.Sprintf("%.2f", sizeMB/1024),
			"file", filepath)
	} else {
		slog.Info("Starting download (unknown size)", "file", filepath)
	}

	// Create progress writer
	start := time.Now()
	var downloaded int64
	
	// Progress reporting function
	reportProgress := func() {
		elapsed := time.Since(start)
		downloadedMB := float64(downloaded) / 1024 / 1024
		speed := downloadedMB / elapsed.Seconds()
		
		if size > 0 {
			progress := float64(downloaded) / float64(size) * 100
			eta := time.Duration(float64(elapsed) * (float64(size)/float64(downloaded) - 1))
			
			slog.Info("Download progress", 
				"progress_percent", fmt.Sprintf("%.1f", progress),
				"downloaded_mb", fmt.Sprintf("%.1f", downloadedMB),
				"total_mb", fmt.Sprintf("%.1f", sizeMB),
				"speed_mbps", fmt.Sprintf("%.2f", speed),
				"eta", eta.Round(time.Second).String(),
				"file", filepath)
		} else {
			slog.Info("Download progress", 
				"downloaded_mb", fmt.Sprintf("%.1f", downloadedMB),
				"speed_mbps", fmt.Sprintf("%.2f", speed),
				"elapsed", elapsed.Round(time.Second).String(),
				"file", filepath)
		}
	}

	// Progress reporting ticker (every 10 seconds)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	// Start progress reporting in background
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				reportProgress()
			case <-done:
				return
			}
		}
	}()

	// Copy with progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			downloaded += int64(n)
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				done <- true
				return writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			done <- true
			return err
		}
	}
	
	// Stop progress reporting
	done <- true
	
	// Final progress report
	elapsed := time.Since(start)
	downloadedMB := float64(downloaded) / 1024 / 1024
	avgSpeed := downloadedMB / elapsed.Seconds()
	
	slog.Info("Download completed", 
		"final_size_mb", fmt.Sprintf("%.1f", downloadedMB),
		"final_size_gb", fmt.Sprintf("%.2f", downloadedMB/1024),
		"duration", elapsed.Round(time.Second).String(),
		"avg_speed_mbps", fmt.Sprintf("%.2f", avgSpeed),
		"file", filepath)
	
	return nil
}

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		if val, ok := v.(float64); ok {
			return int(val)
		}
		if val, ok := v.(int); ok {
			return val
		}
	}
	return defaultVal
}

func getFloatParam(params map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := params[key]; ok {
		if val, ok := v.(float64); ok {
			return val
		}
		if val, ok := v.(int); ok {
			return float64(val)
		}
	}
	return defaultVal
}

func getStringParam(params map[string]interface{}, key string, defaultVal string) string {
	if v, ok := params[key]; ok {
		if val, ok := v.(string); ok {
			return val
		}
	}
	return defaultVal
}