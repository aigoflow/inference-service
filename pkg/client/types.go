package client

import "time"

// InferenceRequest represents a request to the inference service
type InferenceRequest struct {
	ReqID   string                 `json:"req_id"`
	Input   string                 `json:"input"`
	Params  map[string]interface{} `json:"params"`
	Raw     bool                   `json:"raw,omitempty"`
	ReplyTo string                 `json:"reply_to,omitempty"`
}

// InferenceResponse represents a response from the inference service
type InferenceResponse struct {
	ReqID        string `json:"req_id"`
	Text         string `json:"text"`
	TokensIn     int    `json:"tokens_in"`
	TokensOut    int    `json:"tokens_out"`
	FinishReason string `json:"finish_reason"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

// EmbeddingRequest represents a request for embeddings
type EmbeddingRequest struct {
	ReqID   string   `json:"req_id"`
	Input   string   `json:"input"`
	Model   string   `json:"model"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

// EmbeddingData represents a single embedding
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingResponse represents embeddings response
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
	Error  string          `json:"error,omitempty"`
}

// EmbeddingUsage represents token usage
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// HealthStatus represents model health information
type HealthStatus struct {
	ModelName    string    `json:"model_name"`
	Status       string    `json:"status"`
	LastActivity time.Time `json:"last_activity"`
	Capabilities []string  `json:"capabilities"`
	Endpoint     string    `json:"endpoint"`
	NATSTopic    string    `json:"nats_topic"`
	Version      string    `json:"version"`
}