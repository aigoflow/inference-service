package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/oklog/ulid/v2"
)

// InferenceClient provides a client interface for the inference service
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

// NATSInferenceClient implements InferenceClient using NATS (exactly like nats-chat.go)
type NATSInferenceClient struct {
	conn     *nats.Conn
	clientID string
	timeout  time.Duration
}

// NewNATSClient creates a new NATS-based inference client
func NewNATSClient(natsURL, clientID string) (InferenceClient, error) {
	conn, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	
	if clientID == "" {
		clientID = "inference-client"
	}
	
	return &NATSInferenceClient{
		conn:     conn,
		clientID: clientID,
		timeout:  30 * time.Second,
	}, nil
}

// Infer performs text inference using the exact nats-chat.go pattern
func (c *NATSInferenceClient) Infer(ctx context.Context, model, input string, params map[string]interface{}) (*InferenceResponse, error) {
	// Build topic from model name
	topic := fmt.Sprintf("inference.request.%s", model)
	
	// Generate ULID request ID
	reqID := ulid.Make().String()
	replySubject := fmt.Sprintf("inference.response.%s.%s", c.clientID, reqID)
	
	// Create request (exact same structure as nats-chat.go)
	request := InferenceRequest{
		ReqID:   reqID,
		Input:   input,
		Params:  params,
		Raw:     false,
		ReplyTo: replySubject,
	}
	
	return c.sendRequest(ctx, topic, replySubject, request)
}

// InferRaw performs raw inference (bypasses formatting)
func (c *NATSInferenceClient) InferRaw(ctx context.Context, model, input string, params map[string]interface{}) (*InferenceResponse, error) {
	topic := fmt.Sprintf("inference.request.%s", model)
	
	reqID := ulid.Make().String()
	replySubject := fmt.Sprintf("inference.response.%s.%s", c.clientID, reqID)
	
	request := InferenceRequest{
		ReqID:   reqID,
		Input:   input,
		Params:  params,
		Raw:     true,
		ReplyTo: replySubject,
	}
	
	return c.sendRequest(ctx, topic, replySubject, request)
}

// sendRequest implements the exact nats-chat.go pattern
func (c *NATSInferenceClient) sendRequest(ctx context.Context, topic, replySubject string, request InferenceRequest) (*InferenceResponse, error) {
	slog.Debug("Sending inference request",
		"topic", topic,
		"req_id", request.ReqID,
		"reply_subject", replySubject,
		"raw", request.Raw)
	
	// Marshal request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Subscribe to reply subject FIRST (exactly like nats-chat.go)
	replyChan := make(chan *nats.Msg, 1)
	sub, err := c.conn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to reply: %w", err)
	}
	defer sub.Unsubscribe()
	
	// Publish request to topic (exactly like nats-chat.go)
	if err := c.conn.Publish(topic, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}
	
	slog.Debug("Published request, waiting for reply", "reply_subject", replySubject)
	
	// Wait for response with timeout (exactly like nats-chat.go)
	select {
	case msg := <-replyChan:
		slog.Debug("Received response",
			"req_id", request.ReqID,
			"response_size", len(msg.Data))
		
		// Parse response (exactly like nats-chat.go)
		var response InferenceResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		
		return &response, nil
		
	case <-time.After(c.timeout):
		return nil, fmt.Errorf("request timeout after %v", c.timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CheckHealth checks if a model is available and healthy
func (c *NATSInferenceClient) CheckHealth(ctx context.Context, model string) (*HealthStatus, error) {
	healthTopic := fmt.Sprintf("models.%s.health", model)
	
	reqID := ulid.Make().String()
	replySubject := fmt.Sprintf("health.response.%s.%s", c.clientID, reqID)
	
	// Create health request
	healthReq := map[string]interface{}{
		"req_id":   reqID,
		"reply_to": replySubject,
	}
	
	requestBytes, err := json.Marshal(healthReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal health request: %w", err)
	}
	
	// Subscribe to reply subject
	replyChan := make(chan *nats.Msg, 1)
	sub, err := c.conn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to health reply: %w", err)
	}
	defer sub.Unsubscribe()
	
	// Publish health request
	if err := c.conn.Publish(healthTopic, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to publish health request: %w", err)
	}
	
	// Wait for health response
	select {
	case msg := <-replyChan:
		var health HealthStatus
		if err := json.Unmarshal(msg.Data, &health); err != nil {
			return nil, fmt.Errorf("failed to parse health response: %w", err)
		}
		return &health, nil
		
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("health check timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Embed generates embeddings
func (c *NATSInferenceClient) Embed(ctx context.Context, model, input string) (*EmbeddingResponse, error) {
	topic := fmt.Sprintf("embedding.request.%s", model)
	
	reqID := ulid.Make().String()
	replySubject := fmt.Sprintf("embedding.response.%s.%s", c.clientID, reqID)
	
	request := EmbeddingRequest{
		ReqID:   reqID,
		Input:   input,
		Model:   model,
		ReplyTo: replySubject,
	}
	
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}
	
	// Subscribe to reply subject
	replyChan := make(chan *nats.Msg, 1)
	sub, err := c.conn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to embedding reply: %w", err)
	}
	defer sub.Unsubscribe()
	
	// Publish embedding request
	if err := c.conn.Publish(topic, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to publish embedding request: %w", err)
	}
	
	// Wait for embedding response
	select {
	case msg := <-replyChan:
		var response EmbeddingResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse embedding response: %w", err)
		}
		return &response, nil
		
	case <-time.After(c.timeout):
		return nil, fmt.Errorf("embedding request timeout after %v", c.timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ListModels discovers available models via NATS
func (c *NATSInferenceClient) ListModels(ctx context.Context) ([]string, error) {
	discoveryTopic := "models.discovery"
	
	reqID := ulid.Make().String()
	replySubject := fmt.Sprintf("discovery.response.%s.%s", c.clientID, reqID)
	
	request := map[string]interface{}{
		"req_id":   reqID,
		"reply_to": replySubject,
	}
	
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery request: %w", err)
	}
	
	// Subscribe to reply subject
	replyChan := make(chan *nats.Msg, 1)
	sub, err := c.conn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to discovery reply: %w", err)
	}
	defer sub.Unsubscribe()
	
	// Publish discovery request
	if err := c.conn.Publish(discoveryTopic, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to publish discovery request: %w", err)
	}
	
	// Wait for discovery response
	select {
	case msg := <-replyChan:
		var response map[string]interface{}
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse discovery response: %w", err)
		}
		
		// Extract models from response
		if models, ok := response["models"].([]interface{}); ok {
			modelNames := make([]string, len(models))
			for i, model := range models {
				if modelName, ok := model.(string); ok {
					modelNames[i] = modelName
				}
			}
			return modelNames, nil
		}
		
		// Fallback to static list if discovery format is unexpected
		return []string{"gemma3-270m", "qwen3-4b", "gpt-oss-20b"}, nil
		
	case <-time.After(5 * time.Second):
		// Fallback to static list on timeout
		return []string{"gemma3-270m", "qwen3-4b", "gpt-oss-20b"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close closes the NATS connection
func (c *NATSInferenceClient) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

// SetTimeout configures request timeout
func (c *NATSInferenceClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}