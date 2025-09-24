# Inference Service Client Package

Go client library for communicating with the inference service via NATS messaging.

## Installation

```bash
go get github.com/aigoflow/inference-service/pkg/client
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/aigoflow/inference-service/pkg/client"
)

func main() {
    // Create client
    client, err := client.NewNATSClient("nats://127.0.0.1:5700", "my-service")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    ctx := context.Background()
    
    // Text inference
    response, err := client.Infer(ctx, "gemma3-270m", "What is AI?", map[string]interface{}{
        "temperature": 0.7,
        "max_tokens": 100,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Response: %s", response.Text)
}
```

## Interface

```go
type InferenceClient interface {
    // Text inference
    Infer(ctx context.Context, model, input string, params map[string]interface{}) (*InferenceResponse, error)
    InferRaw(ctx context.Context, model, input string, params map[string]interface{}) (*InferenceResponse, error)
    
    // Embeddings
    Embed(ctx context.Context, model, input string) (*EmbeddingResponse, error)
    
    // Health and discovery
    CheckHealth(ctx context.Context, model string) (*HealthStatus, error)
    ListModels(ctx context.Context) ([]string, error)
    
    // Lifecycle
    Close() error
}
```

## Methods

### Text Inference

**Formatted inference** (uses model templates):
```go
response, err := client.Infer(ctx, "gemma3-270m", "Hello", params)
```

**Raw inference** (bypasses formatting):
```go
response, err := client.InferRaw(ctx, "gemma3-270m", "raw input", params)
```

### Health Checks

```go
health, err := client.CheckHealth(ctx, "gemma3-270m")
if err != nil {
    log.Printf("Model unavailable: %v", err)
} else {
    log.Printf("Model %s is %s", health.ModelName, health.Status)
}
```

### Model Discovery

```go
models, err := client.ListModels(ctx)
if err != nil {
    log.Printf("Discovery failed: %v", err)
} else {
    log.Printf("Available models: %v", models)
}
```

### Embeddings

```go
embedding, err := client.Embed(ctx, "nomic-embed-v1.5", "Hello world")
if err != nil {
    log.Printf("Embedding failed: %v", err)
} else {
    log.Printf("Generated %d-dimensional embedding", len(embedding.Data[0].Embedding))
}
```

## Configuration

### Timeout Configuration

```go
import "time"

// Create client
client, err := client.NewNATSClient(natsURL, clientID)
if err != nil {
    log.Fatal(err)
}

// Configure timeout (default: 30 seconds)
if natsClient, ok := client.(*client.NATSInferenceClient); ok {
    natsClient.SetTimeout(60 * time.Second)
}
```

### NATS Connection Options

```go
// Custom client ID for reply subjects
client, err := client.NewNATSClient("nats://127.0.0.1:5700", "reasoning-service")

// Default client ID
client, err := client.NewNATSClient("nats://127.0.0.1:5700", "")  // Uses "inference-client"
```

## Error Handling

```go
response, err := client.Infer(ctx, "gemma3-270m", "Hello", params)
if err != nil {
    // Network or NATS errors
    log.Printf("Request failed: %v", err)
    return
}

if response.Error != "" {
    // Model or inference errors
    log.Printf("Inference error: %s", response.Error)
    return
}

// Success
log.Printf("Response: %s", response.Text)
```

## Response Types

### InferenceResponse

```go
type InferenceResponse struct {
    ReqID        string `json:"req_id"`
    Text         string `json:"text"`
    TokensIn     int    `json:"tokens_in"`
    TokensOut    int    `json:"tokens_out"`
    FinishReason string `json:"finish_reason"`
    DurationMs   int64  `json:"duration_ms"`
    Error        string `json:"error,omitempty"`
}
```

### HealthStatus

```go
type HealthStatus struct {
    ModelName    string    `json:"model_name"`
    Status       string    `json:"status"`
    LastActivity time.Time `json:"last_activity"`
    Capabilities []string  `json:"capabilities"`
    Endpoint     string    `json:"endpoint"`
    NATSTopic    string    `json:"nats_topic"`
    Version      string    `json:"version"`
}
```

### EmbeddingResponse

```go
type EmbeddingResponse struct {
    Object string          `json:"object"`
    Data   []EmbeddingData `json:"data"`
    Model  string          `json:"model"`
    Usage  EmbeddingUsage  `json:"usage"`
    Error  string          `json:"error,omitempty"`
}
```

## Examples

See `../../examples/demo-client.go` for a comprehensive example showcasing all client features.

## NATS Topics

The client automatically handles NATS topic routing:

- **Inference**: `inference.request.{model}` → `inference.response.{client-id}.{req-id}`
- **Health**: `models.{model}.health` → `health.response.{client-id}.{req-id}`
- **Discovery**: `models.discovery` → `discovery.response.{client-id}.{req-id}`
- **Embeddings**: `embedding.request.{model}` → `embedding.response.{client-id}.{req-id}`

## Integration Example

```go
// Service using the client
type MyService struct {
    inferenceClient client.InferenceClient
}

func NewMyService(natsURL string) (*MyService, error) {
    inferenceClient, err := client.NewNATSClient(natsURL, "my-service")
    if err != nil {
        return nil, err
    }
    
    return &MyService{
        inferenceClient: inferenceClient,
    }, nil
}

func (s *MyService) ProcessRequest(ctx context.Context, input string) (string, error) {
    // Check model health
    health, err := s.inferenceClient.CheckHealth(ctx, "gemma3-270m")
    if err != nil || health.Status != "online" {
        return "", fmt.Errorf("model unavailable")
    }
    
    // Make inference request
    response, err := s.inferenceClient.Infer(ctx, "gemma3-270m", input, map[string]interface{}{
        "temperature": 0.7,
        "max_tokens": 100,
    })
    if err != nil {
        return "", err
    }
    
    if response.Error != "" {
        return "", fmt.Errorf("inference error: %s", response.Error)
    }
    
    return response.Text, nil
}

func (s *MyService) Close() error {
    return s.inferenceClient.Close()
}
```