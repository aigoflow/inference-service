package capabilities

// CapabilityType represents different types of AI capabilities
type CapabilityType string

const (
	CapabilityTextGeneration     CapabilityType = "text-generation"
	CapabilityEmbeddings         CapabilityType = "embeddings"
	CapabilityImageGeneration    CapabilityType = "image-generation"
	CapabilityImageUnderstanding CapabilityType = "image-understanding"
	CapabilityAudioGeneration    CapabilityType = "audio-generation"
	CapabilityAudioTranscription CapabilityType = "audio-transcription"
	CapabilityVideoUnderstanding CapabilityType = "video-understanding"
	CapabilityGrammarConstrained CapabilityType = "grammar-constrained"
	CapabilityReasoning          CapabilityType = "reasoning"
	CapabilityToolCalling        CapabilityType = "tool-calling"
)

// Capability represents a specific AI capability with metadata
type Capability struct {
	Type        CapabilityType         `json:"type"`
	Version     string                 `json:"version"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// ModelMetadata contains detailed information about the model
type ModelMetadata struct {
	Architecture    string                 `json:"architecture"`
	Modalities      []string               `json:"modalities"`
	ParameterCount  string                 `json:"parameter_count,omitempty"`
	ContextSize     int                    `json:"context_size,omitempty"`
	EmbeddingSize   int                    `json:"embedding_size,omitempty"`
	Quantization    string                 `json:"quantization,omitempty"`
	ModelFamily     string                 `json:"model_family,omitempty"`
	Additional      map[string]interface{} `json:"additional,omitempty"`
}

// ModelInterface defines the interface for model introspection
type ModelInterface interface {
	// Existing embedding methods
	IsEmbeddingModel() bool
	GetEmbeddingSize() int

	// New introspection methods
	GetModelArchitecture() string
	GetSupportedModalities() []string
	GetModelMetadata() ModelMetadata
	HasCapability(capability string) bool
}

// CapabilityDetector interface for detecting model capabilities
type CapabilityDetector interface {
	DetectCapabilities(model ModelInterface) []Capability
	SupportsCapability(model ModelInterface, capability CapabilityType) bool
	GetCapabilityStrings(capabilities []Capability) []string
}