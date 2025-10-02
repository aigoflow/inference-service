package whisper

/*
#cgo CXXFLAGS: -I${SRCDIR}/../../whisper.cpp/include -I${SRCDIR}/../../whisper.cpp/ggml/include -std=c++11
#cgo LDFLAGS: -L${SRCDIR} -L${SRCDIR}/../../whisper.cpp/build/src -lwhisper -lm
#cgo darwin LDFLAGS: -framework Accelerate -framework Foundation -framework Metal -framework MetalKit -framework MetalPerformanceShaders
#cgo linux LDFLAGS: -lstdc++
#include "binding.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"log/slog"
	"net/http"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"
	
	"github.com/aigoflow/inference-service/internal/config"
	"github.com/aigoflow/inference-service/internal/types"
)

// WhisperModel represents a Whisper model instance
type WhisperModel struct {
	model  unsafe.Pointer
	config *config.Config
}

// Config holds Whisper-specific configuration
type WhisperConfig struct {
	ModelPath string
	Language  string
	Threads   int
}

// NewWhisperModel creates a new Whisper model instance
func NewWhisperModel(cfg *config.Config) (*WhisperModel, error) {
	modelPath := C.CString(cfg.ModelPath)
	defer C.free(unsafe.Pointer(modelPath))
	
	model := C.whisper_load_model_from_file(modelPath)
	if model == nil {
		return nil, fmt.Errorf("failed to load Whisper model from %s", cfg.ModelPath)
	}
	
	slog.Info("Whisper model loaded successfully", "path", cfg.ModelPath)
	
	return &WhisperModel{
		model:  model,
		config: cfg,
	}, nil
}

// Transcribe performs audio transcription from uploaded file data
func (w *WhisperModel) Transcribe(audioData []byte, language string) (string, []types.AudioSegment, error) {
	slog.Info("Whisper transcribe called", "audio_size", len(audioData), "language", language)
	if w.model == nil {
		return "", nil, fmt.Errorf("whisper model not loaded")
	}
	
	// Create temporary file for audio data
	tmpFile, err := os.CreateTemp("", "whisper_audio_*.wav")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	
	// Write audio data to temp file
	if _, err := tmpFile.Write(audioData); err != nil {
		return "", nil, fmt.Errorf("failed to write audio data: %w", err)
	}
	tmpFile.Close()
	
	// Check if file is MP3 and convert to WAV if needed
	audioFile := tmpFile.Name()
	if isMP3File(audioData) {
		wavFile := tmpFile.Name() + ".wav"
		defer os.Remove(wavFile)
		
		if err := convertMP3ToWAV(tmpFile.Name(), wavFile); err != nil {
			return "", nil, fmt.Errorf("failed to convert MP3 to WAV: %w", err)
		}
		audioFile = wavFile
	}
	
	// Set language
	var langPtr *C.char
	if language != "" && language != "auto" {
		langPtr = C.CString(language)
		defer C.free(unsafe.Pointer(langPtr))
	}
	
	// Prepare result buffers
	const maxSegments = 256
	const maxTextLen = 8192
	
	result := make([]byte, maxTextLen)
	segmentStarts := make([]C.float, maxSegments)
	segmentEnds := make([]C.float, maxSegments)
	segmentTexts := make([]*C.char, maxSegments)
	
	// Convert audio to float32 data and use CGO bindings
	audioFloat32, err := w.convertAudioToFloat32(audioFile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert audio: %w", err)
	}
	
	// Call transcription with converted audio data
	slog.Info("Calling whisper CGO", "audio_float32_len", len(audioFloat32), "lang", language)
	numSegments := C.whisper_transcribe_wrapper(
		w.model,
		(*C.float)(unsafe.Pointer(&audioFloat32[0])),
		C.int(len(audioFloat32)),
		langPtr,
		(*C.char)(unsafe.Pointer(&result[0])),
		C.int(maxTextLen),
		&segmentStarts[0],
		&segmentEnds[0],
		(**C.char)(unsafe.Pointer(&segmentTexts[0])),
		C.int(maxSegments),
	)
	slog.Info("Whisper CGO returned", "num_segments", int(numSegments))
	
	if numSegments < 0 {
		return "", nil, fmt.Errorf("whisper transcription failed")
	}
	
	// Build result
	fullText := C.GoString((*C.char)(unsafe.Pointer(&result[0])))
	
	// Build segments
	segments := make([]types.AudioSegment, numSegments)
	for i := 0; i < int(numSegments); i++ {
		segmentText := C.GoString(segmentTexts[i])
		C.free(unsafe.Pointer(segmentTexts[i])) // Free allocated string
		
		segments[i] = types.AudioSegment{
			ID:    i,
			Start: float64(segmentStarts[i]),
			End:   float64(segmentEnds[i]),
			Text:  segmentText,
		}
	}
	
	return fullText, segments, nil
}

// isMP3File checks if the file data appears to be MP3 format
func isMP3File(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	// Check for MP3 header signatures
	return (data[0] == 0xFF && (data[1]&0xE0) == 0xE0) || // MPEG header
		   (len(data) >= 3 && string(data[0:3]) == "ID3") // ID3 tag
}

// convertMP3ToWAV converts MP3 file to WAV using ffmpeg with correct format for Whisper
func convertMP3ToWAV(mp3Path, wavPath string) error {
	// Based on search results: whisper.cpp needs 16kHz, 16-bit PCM WAV files
	cmd := exec.Command("ffmpeg", "-i", mp3Path, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wavPath, "-y")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w, output: %s", err, string(output))
	}
	
	return nil
}

// convertAudioToFloat32 converts audio file to float32 PCM data for whisper.cpp
func (w *WhisperModel) convertAudioToFloat32(audioFile string) ([]float32, error) {
	// Convert to raw float32 PCM data for whisper.cpp
	tmpFile := audioFile + ".f32"
	defer os.Remove(tmpFile)
	
	// Use ffmpeg to convert to raw float32 PCM data
	cmd := exec.Command("ffmpeg", "-i", audioFile, "-ar", "16000", "-ac", "1", "-f", "f32le", tmpFile, "-y")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg float32 conversion failed: %w, output: %s", err, string(output))
	}
	
	// Read the float32 data
	f32Data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read float32 data: %w", err)
	}
	
	// Convert bytes to float32 slice
	float32Count := len(f32Data) / 4
	float32Data := make([]float32, float32Count)
	
	for i := 0; i < float32Count; i++ {
		// Read 4 bytes as float32 (little endian)
		bits := uint32(f32Data[i*4]) | uint32(f32Data[i*4+1])<<8 | uint32(f32Data[i*4+2])<<16 | uint32(f32Data[i*4+3])<<24
		float32Data[i] = *(*float32)(unsafe.Pointer(&bits))
	}
	
	return float32Data, nil
}

// GetModelName returns the model name
func (w *WhisperModel) GetModelName() string {
	return w.config.ModelName
}

// GetLanguages returns supported languages
func (w *WhisperModel) GetLanguages() []string {
	const maxLangs = 32
	langPtrs := make([]*C.char, maxLangs)
	
	numLangs := C.whisper_get_supported_languages_wrapper(w.model, (**C.char)(unsafe.Pointer(&langPtrs[0])), maxLangs)
	
	languages := make([]string, numLangs)
	for i := 0; i < int(numLangs); i++ {
		languages[i] = C.GoString(langPtrs[i])
		C.free(unsafe.Pointer(langPtrs[i]))
	}
	
	return languages
}

// IsAudioModel returns true for Whisper models
func (w *WhisperModel) IsAudioModel() bool {
	return true
}

// GetEmbeddingSize returns 0 as Whisper models don't support embeddings
func (w *WhisperModel) GetEmbeddingSize() int {
	return 0
}

// GetModelArchitecture returns the model architecture
func (w *WhisperModel) GetModelArchitecture() string {
	return "whisper"
}

// GetSupportedModalities returns supported modalities
func (w *WhisperModel) GetSupportedModalities() []string {
	return []string{"audio"}
}

// GetModelMetadata returns model metadata
func (w *WhisperModel) GetModelMetadata() interface{} {
	return map[string]interface{}{
		"name":         w.config.ModelName,
		"architecture": "whisper",
		"modalities":   []string{"audio"},
		"audio_model":  true,
	}
}

// HasCapability checks if the model has a specific capability
func (w *WhisperModel) HasCapability(capability string) bool {
	return capability == "audio_transcription"
}

// IsEmbeddingModel returns false for Whisper models
func (w *WhisperModel) IsEmbeddingModel() bool {
	return false
}

// Close frees the model resources
func (w *WhisperModel) Close() {
	if w.model != nil {
		C.whisper_free_model_wrapper(w.model)
		w.model = nil
	}
}

// LoadWithAutoDownload loads a Whisper model with automatic download if missing
func LoadWithAutoDownload(modelPath, modelURL string, cfg *config.Config) (*WhisperModel, error) {
	// Check if model file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		if modelURL == "" {
			return nil, fmt.Errorf("model file not found and no download URL provided: %s", modelPath)
		}
		
		slog.Info("Downloading Whisper model", "url", modelURL, "path", modelPath)
		
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create model directory: %w", err)
		}
		
		// Download the model
		if err := downloadFile(modelURL, modelPath); err != nil {
			return nil, fmt.Errorf("failed to download model: %w", err)
		}
		
		slog.Info("Model downloaded successfully", "path", modelPath)
	}
	
	// Load the model
	return NewWhisperModel(cfg)
}

// downloadFile downloads a file from url to filepath
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
	
	_, err = io.Copy(out, resp.Body)
	return err
}