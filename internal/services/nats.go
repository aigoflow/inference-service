package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/aigoflow/inference-service/internal/config"
	"github.com/aigoflow/inference-service/internal/repository"
)

// generateWorkerID creates a unique worker ID using timestamp and random bytes
func generateWorkerID() string {
	// Use timestamp + random bytes for uniqueness
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("worker-%d-%s", timestamp, randomHex)
}

type ServiceInterface interface {
	GetRepository() repository.Repository
}

type NATSService struct {
	conn             *nats.Conn
	js               nats.JetStreamContext
	service          ServiceInterface
	inferenceService *InferenceService
	embeddingService *EmbeddingService
	audioService     *AudioService
	cfg              *config.Config
	monitoring       *MonitoringService
}

func NewNATSService(cfg *config.Config, service ServiceInterface) (*NATSService, error) {
	// Connect to NATS
	conn, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	natsService := &NATSService{
		conn:       conn,
		js:         js,
		service:    service,
		cfg:        cfg,
		monitoring: NewMonitoringService(conn, cfg),
	}

	// Set specific service types for backward compatibility
	if inferenceService, ok := service.(*InferenceService); ok {
		natsService.inferenceService = inferenceService
	} else if audioService, ok := service.(*AudioService); ok {
		natsService.audioService = audioService
	}

	return natsService, nil
}

func (s *NATSService) Start(ctx context.Context) error {
	// Create or update stream
	if err := s.ensureStream(); err != nil {
		return fmt.Errorf("failed to ensure stream: %w", err)
	}

	// Create pull consumer
	consumer, err := s.createConsumer()
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	slog.Info("NATS service starting", 
		"stream", s.cfg.Stream,
		"subject", s.cfg.Subject,
		"consumer", s.cfg.Durable,
		"concurrency", s.cfg.Concurrency)

	// Start monitoring service
	go s.monitoring.Start(ctx)
	
	// Start workers with unique IDs
	for i := 0; i < s.cfg.Concurrency; i++ {
		workerID := generateWorkerID()
		go s.worker(ctx, consumer, workerID)
	}

	// Block until context is cancelled
	<-ctx.Done()
	slog.Info("NATS service shutting down")
	
	// Close connection
	s.conn.Close()
	return nil
}

func (s *NATSService) ensureStream() error {
	streamInfo, err := s.js.StreamInfo(s.cfg.Stream)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			// Create new stream
			_, err = s.js.AddStream(&nats.StreamConfig{
				Name:      s.cfg.Stream,
				Subjects:  []string{s.cfg.Subject},
				MaxMsgs:   int64(s.cfg.MaxMsgs),
				MaxAge:    s.cfg.MaxAge,
				Storage:   nats.FileStorage,
				Retention: nats.WorkQueuePolicy,
			})
			if err != nil {
				return fmt.Errorf("failed to create stream: %w", err)
			}
			slog.Info("Created NATS stream", "name", s.cfg.Stream)
		} else {
			return fmt.Errorf("failed to get stream info: %w", err)
		}
	} else {
		// Check if stream has our subject, update if needed
		hasSubject := false
		for _, subject := range streamInfo.Config.Subjects {
			if subject == s.cfg.Subject {
				hasSubject = true
				break
			}
		}
		
		if !hasSubject {
			// Update stream to include our subject
			newConfig := streamInfo.Config
			newConfig.Subjects = append(newConfig.Subjects, s.cfg.Subject)
			_, err = s.js.UpdateStream(&newConfig)
			if err != nil {
				return fmt.Errorf("failed to update stream with new subject: %w", err)
			}
			slog.Info("Updated NATS stream with new subject", "name", s.cfg.Stream, "subject", s.cfg.Subject)
		} else {
			slog.Info("NATS stream already exists", "name", s.cfg.Stream, "messages", streamInfo.State.Msgs)
		}
	}

	return nil
}

func (s *NATSService) createConsumer() (*nats.Subscription, error) {
	// Create pull consumer
	sub, err := s.js.PullSubscribe(s.cfg.Subject, s.cfg.Durable, nats.ManualAck())
	if err != nil {
		return nil, fmt.Errorf("failed to create pull consumer: %w", err)
	}

	slog.Info("Created NATS consumer", "durable", s.cfg.Durable)
	return sub, nil
}

func (s *NATSService) worker(ctx context.Context, consumer *nats.Subscription, workerID string) {
	slog.Info("NATS worker starting", "worker_id", workerID)
	
	for {
		select {
		case <-ctx.Done():
			slog.Info("NATS worker shutting down", "worker_id", workerID)
			return
		default:
			// Fetch messages with timeout
			msgs, err := consumer.Fetch(1, nats.MaxWait(time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue // Normal timeout, continue polling
				}
				slog.Error("Failed to fetch messages", "worker_id", workerID, "error", err)
				time.Sleep(time.Second) // Back off on error
				continue
			}

			for _, msg := range msgs {
				// Track message processing
				s.monitoring.IncrementPending()
				s.processMessage(ctx, msg, workerID)
				s.monitoring.DecrementPending()
			}
		}
	}
}

func (s *NATSService) processMessage(ctx context.Context, msg *nats.Msg, workerID string) {
	// Track active processing
	s.monitoring.IncrementActive()
	defer s.monitoring.DecrementActive()
	
	// Determine the request type based on subject
	isEmbeddingRequest := strings.Contains(msg.Subject, "embedding.request")
	isAudioRequest := strings.Contains(msg.Subject, "audio.request") || strings.Contains(msg.Subject, "transcribe.request")
	
	if isEmbeddingRequest {
		s.processEmbeddingMessage(ctx, msg, workerID)
	} else if isAudioRequest {
		s.processAudioMessage(ctx, msg, workerID)
	} else {
		s.processInferenceMessage(ctx, msg, workerID)
	}
}

func (s *NATSService) processInferenceMessage(ctx context.Context, msg *nats.Msg, workerID string) {
	start := time.Now()
	
	// Parse inference request
	var req InferenceRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		slog.Error("Failed to parse inference request", 
			"worker_id", workerID, 
			"error", err,
			"data", string(msg.Data))
		msg.Nak() // Negative acknowledgment
		return
	}

	// Generate trace ID if not provided
	if req.TraceID == "" {
		req.TraceID = req.ReqID
	}

	slog.Debug("Processing NATS inference request",
		"worker_id", workerID,
		"req_id", req.ReqID,
		"trace_id", req.TraceID,
		"subject", msg.Subject)

	// Process inference using the same service
	response, err := s.inferenceService.ProcessInference(
		ctx, 
		req, 
		fmt.Sprintf("nats.%s", msg.Subject), 
		req.ReplyTo, // Use reply_to from message payload, not msg.Reply
		workerID,
	)

	// Prepare response
	responseData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		slog.Error("Failed to marshal response", 
			"worker_id", workerID,
			"req_id", req.ReqID, 
			"error", marshalErr)
		msg.Nak()
		return
	}

	// Send response if reply subject is provided in message payload
	if req.ReplyTo != "" {
		if publishErr := s.conn.Publish(req.ReplyTo, responseData); publishErr != nil {
			slog.Error("Failed to publish response", 
				"worker_id", workerID,
				"req_id", req.ReqID,
				"reply_subject", req.ReplyTo, 
				"error", publishErr)
		}
	}

	// Acknowledge message
	if ackErr := msg.Ack(); ackErr != nil {
		slog.Error("Failed to acknowledge message", 
			"worker_id", workerID,
			"req_id", req.ReqID, 
			"error", ackErr)
	}

	duration := time.Since(start)
	
	// Log successful processing
	if err == nil {
		slog.Info("NATS inference completed",
			"worker_id", workerID,
			"req_id", req.ReqID,
			"duration_ms", duration.Milliseconds(),
			"tokens_in", response.TokensIn,
			"tokens_out", response.TokensOut)
	} else {
		slog.Error("NATS inference failed",
			"worker_id", workerID,
			"req_id", req.ReqID,
			"duration_ms", duration.Milliseconds(),
			"error", err)
	}
}

func (s *NATSService) processAudioMessage(ctx context.Context, msg *nats.Msg, workerID string) {
	start := time.Now()
	
	// Parse audio request
	var req AudioRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		slog.Error("Failed to parse audio request", 
			"worker_id", workerID, 
			"error", err,
			"data", string(msg.Data))
		msg.Nak() // Negative acknowledgment
		return
	}

	// Generate trace ID if not provided
	if req.TraceID == "" {
		req.TraceID = req.ReqID
	}

	slog.Debug("Processing NATS audio request",
		"worker_id", workerID,
		"req_id", req.ReqID,
		"trace_id", req.TraceID,
		"subject", msg.Subject)

	// Audio service should be set during initialization, but handle the case where it's not
	if s.audioService == nil {
		slog.Error("Audio service not initialized - cannot process audio request", "worker_id", workerID)
		msg.Nak()
		return
	}

	// Process audio request
	response, err := s.audioService.ProcessTranscription(
		ctx, 
		req, 
		fmt.Sprintf("nats.%s", msg.Subject), 
		req.ReplyTo,
		workerID,
	)

	// Prepare response
	responseData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		slog.Error("Failed to marshal audio response", 
			"worker_id", workerID,
			"req_id", req.ReqID, 
			"error", marshalErr)
		msg.Nak()
		return
	}

	// Send response if reply subject is provided in message payload
	if req.ReplyTo != "" {
		if publishErr := s.conn.Publish(req.ReplyTo, responseData); publishErr != nil {
			slog.Error("Failed to publish audio response", 
				"worker_id", workerID,
				"req_id", req.ReqID,
				"reply_subject", req.ReplyTo, 
				"error", publishErr)
		}
	}

	// Acknowledge message
	if ackErr := msg.Ack(); ackErr != nil {
		slog.Error("Failed to acknowledge audio message", 
			"worker_id", workerID,
			"req_id", req.ReqID, 
			"error", ackErr)
	}

	duration := time.Since(start)
	
	// Log successful processing
	if err == nil {
		slog.Info("NATS audio transcription completed",
			"worker_id", workerID,
			"req_id", req.ReqID,
			"duration_ms", duration.Milliseconds(),
			"segments_count", len(response.Segments))
	} else {
		slog.Error("NATS audio transcription failed",
			"worker_id", workerID,
			"req_id", req.ReqID,
			"duration_ms", duration.Milliseconds(),
			"error", err)
	}
}

func (s *NATSService) processEmbeddingMessage(ctx context.Context, msg *nats.Msg, workerID string) {
	// Parse embedding request
	var req EmbeddingRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		slog.Error("Failed to parse embedding request", 
			"worker_id", workerID, 
			"error", err,
			"data", string(msg.Data))
		msg.Nak() // Negative acknowledgment
		return
	}

	// Generate trace ID if not provided
	if req.TraceID == "" {
		req.TraceID = req.ReqID
	}

	slog.Debug("Processing NATS embedding request",
		"worker_id", workerID,
		"req_id", req.ReqID,
		"trace_id", req.TraceID,
		"subject", msg.Subject)

	// Create embedding service if not already created
	if s.embeddingService == nil {
		s.embeddingService = NewEmbeddingService(s.inferenceService.llm, s.inferenceService.GetRepository())
	}

	// Process embedding request
	response, err := s.embeddingService.ProcessEmbedding(
		ctx, 
		req, 
		fmt.Sprintf("nats.%s", msg.Subject), 
		req.ReplyTo,
		workerID,
	)

	// Prepare response
	responseData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		slog.Error("Failed to marshal embedding response", 
			"worker_id", workerID,
			"req_id", req.ReqID, 
			"error", marshalErr)
		msg.Nak()
		return
	}

	// Send response if reply subject is provided in message payload
	if req.ReplyTo != "" {
		if publishErr := s.conn.Publish(req.ReplyTo, responseData); publishErr != nil {
			slog.Error("Failed to publish embedding response", 
				"worker_id", workerID,
				"req_id", req.ReqID,
				"reply_subject", req.ReplyTo, 
				"error", publishErr)
		}
	}

	// Acknowledge message
	if ackErr := msg.Ack(); ackErr != nil {
		slog.Error("Failed to acknowledge embedding message", 
			"worker_id", workerID,
			"req_id", req.ReqID, 
			"error", ackErr)
	}

	// Log successful processing
	if err == nil {
		slog.Info("NATS embedding completed",
			"worker_id", workerID,
			"req_id", req.ReqID,
			"embedding_count", len(response.Data))
	} else {
		slog.Error("NATS embedding failed",
			"worker_id", workerID,
			"req_id", req.ReqID,
			"error", err)
	}
}

func (s *NATSService) Close() error {
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

func (s *NATSService) GetConnection() *nats.Conn {
	return s.conn
}

func (s *NATSService) GetMonitoringService() *MonitoringService {
	return s.monitoring
}

func (s *NATSService) SetAudioService(audioService *AudioService) {
	s.audioService = audioService
}

func (s *NATSService) GetAudioService() *AudioService {
	return s.audioService
}