package capabilities

import (
	"fmt"
	"log/slog"
	"strings"
)

// AutoCapabilityDetector implements automatic capability detection from models
type AutoCapabilityDetector struct{}

// NewAutoCapabilityDetector creates a new auto-capability detector
func NewAutoCapabilityDetector() *AutoCapabilityDetector {
	return &AutoCapabilityDetector{}
}

// DetectCapabilities automatically detects all capabilities supported by the model
func (d *AutoCapabilityDetector) DetectCapabilities(model ModelInterface) []Capability {
	var capabilities []Capability

	// Always has text generation
	capabilities = append(capabilities, Capability{
		Type:        CapabilityTextGeneration,
		Version:     "1.0",
		Description: "Generate text completions",
	})

	// Check embeddings capability
	if model.IsEmbeddingModel() {
		params := map[string]interface{}{
			"dimension": model.GetEmbeddingSize(),
		}
		capabilities = append(capabilities, Capability{
			Type:        CapabilityEmbeddings,
			Version:     "1.0",
			Parameters:  params,
			Description: "Generate text embeddings",
		})
		slog.Debug("Detected embeddings capability", "dimension", model.GetEmbeddingSize())
	}

	// Check multimodal capabilities
	modalities := model.GetSupportedModalities()
	for _, modality := range modalities {
		switch modality {
		case "image":
			capabilities = append(capabilities, Capability{
				Type:        CapabilityImageUnderstanding,
				Version:     "1.0",
				Description: "Understand and analyze images",
			})
			slog.Debug("Detected image understanding capability")

		case "audio":
			capabilities = append(capabilities, Capability{
				Type:        CapabilityAudioTranscription,
				Version:     "1.0",
				Description: "Transcribe audio to text",
			})
			slog.Debug("Detected audio transcription capability")
		}
	}

	// Check for reasoning capability
	if d.supportsReasoning(model) {
		capabilities = append(capabilities, Capability{
			Type:        CapabilityReasoning,
			Version:     "1.0",
			Description: "Advanced reasoning and problem solving",
		})
		slog.Debug("Detected reasoning capability")
	}

	// Grammar-constrained generation (all text models support this)
	capabilities = append(capabilities, Capability{
		Type:        CapabilityGrammarConstrained,
		Version:     "1.0",
		Description: "Generate text following specific grammar rules",
	})

	// Tool calling capability (detect based on architecture/family)
	if d.supportsToolCalling(model) {
		capabilities = append(capabilities, Capability{
			Type:        CapabilityToolCalling,
			Version:     "1.0",
			Description: "Call external tools and functions",
		})
		slog.Debug("Detected tool calling capability")
	}

	slog.Info("Capability detection completed", 
		"model", model.GetModelMetadata().Additional["config_name"],
		"architecture", model.GetModelArchitecture(),
		"total_capabilities", len(capabilities))

	return capabilities
}

// SupportsCapability checks if a model supports a specific capability
func (d *AutoCapabilityDetector) SupportsCapability(model ModelInterface, capability CapabilityType) bool {
	capabilities := d.DetectCapabilities(model)
	
	for _, cap := range capabilities {
		if cap.Type == capability {
			return true
		}
	}
	
	return false
}

// GetCapabilityStrings converts capabilities to string array for JSON serialization
func (d *AutoCapabilityDetector) GetCapabilityStrings(capabilities []Capability) []string {
	strings := make([]string, len(capabilities))
	for i, cap := range capabilities {
		strings[i] = string(cap.Type)
	}
	return strings
}

// supportsReasoning checks if the model is known to support advanced reasoning
func (d *AutoCapabilityDetector) supportsReasoning(model ModelInterface) bool {
	metadata := model.GetModelMetadata()
	arch := strings.ToLower(metadata.Architecture)
	family := strings.ToLower(metadata.ModelFamily)
	
	// Known reasoning-capable architectures
	reasoningArchs := []string{
		"gpt", "gemma", "qwen", "llama", "phi", "mistral", "claude",
		"o1", "deepseek", "yi", "baichuan", "internlm", "chatglm",
	}
	
	for _, reasoningArch := range reasoningArchs {
		if strings.Contains(arch, reasoningArch) || strings.Contains(family, reasoningArch) {
			return true
		}
	}
	
	// Also check parameter count - larger models typically have better reasoning
	paramStr := metadata.ParameterCount
	if strings.HasSuffix(paramStr, "B") {
		// Models with 1B+ parameters usually support reasoning
		return true
	}
	
	return false
}

// supportsToolCalling checks if the model supports tool/function calling
func (d *AutoCapabilityDetector) supportsToolCalling(model ModelInterface) bool {
	metadata := model.GetModelMetadata()
	arch := strings.ToLower(metadata.Architecture)
	family := strings.ToLower(metadata.ModelFamily)
	
	// Check model name for tool calling indicators
	if modelName, ok := metadata.Additional["model_name"].(string); ok {
		modelNameLower := strings.ToLower(modelName)
		if strings.Contains(modelNameLower, "tool") || 
		   strings.Contains(modelNameLower, "function") ||
		   strings.Contains(modelNameLower, "instruct") {
			return true
		}
	}
	
	// Known tool-calling capable models/families
	toolArchs := []string{
		"gpt-4", "gpt-3.5", "claude", "gemini", "qwen", "deepseek",
		"yi", "mistral", "llama-3", "phi-3",
	}
	
	for _, toolArch := range toolArchs {
		if strings.Contains(arch, toolArch) || strings.Contains(family, toolArch) {
			return true
		}
	}
	
	return false
}

// GetCapabilitiesSummary returns a human-readable summary of capabilities
func (d *AutoCapabilityDetector) GetCapabilitiesSummary(capabilities []Capability) string {
	var summary []string
	
	for _, cap := range capabilities {
		switch cap.Type {
		case CapabilityTextGeneration:
			summary = append(summary, "Text Generation")
		case CapabilityEmbeddings:
			if dim, ok := cap.Parameters["dimension"].(int); ok && dim > 0 {
				summary = append(summary, fmt.Sprintf("Embeddings (%dD)", dim))
			} else {
				summary = append(summary, "Embeddings")
			}
		case CapabilityImageUnderstanding:
			summary = append(summary, "Vision")
		case CapabilityAudioTranscription:
			summary = append(summary, "Audio")
		case CapabilityReasoning:
			summary = append(summary, "Reasoning")
		case CapabilityGrammarConstrained:
			summary = append(summary, "Grammar")
		case CapabilityToolCalling:
			summary = append(summary, "Tool Calling")
		}
	}
	
	if len(summary) == 0 {
		return "Text Only"
	}
	
	return strings.Join(summary, ", ")
}