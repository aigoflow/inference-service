package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/aigoflow/inference-service/internal/config"
)

type HealthService struct {
	nats   *nats.Conn
	config *config.Config
}

type HealthStatus struct {
	ModelName    string    `json:"model_name"`
	Status       string    `json:"status"`       // online, offline, busy
	LastActivity time.Time `json:"last_activity"`
	Capabilities []string  `json:"capabilities"`
	Endpoint     string    `json:"endpoint"`
	NATSTopic    string    `json:"nats_topic"`
	Version      string    `json:"version"`
}

func NewHealthService(natsConn *nats.Conn, cfg *config.Config) *HealthService {
	return &HealthService{
		nats:   natsConn,
		config: cfg,
	}
}

func (h *HealthService) Start(ctx context.Context) error {
	// Subscribe to health check requests for this model
	healthTopic := fmt.Sprintf("models.%s.health", h.config.ModelName)
	
	_, err := h.nats.Subscribe(healthTopic, func(msg *nats.Msg) {
		status := h.getHealthStatus()
		
		statusData, err := json.Marshal(status)
		if err != nil {
			slog.Error("Failed to marshal health status", "error", err)
			return
		}
		
		if err := msg.Respond(statusData); err != nil {
			slog.Error("Failed to respond to health check", "error", err)
		}
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to health topic: %w", err)
	}
	
	slog.Info("Health service started", "topic", healthTopic)
	
	// Publish periodic heartbeats
	go h.publishHeartbeats(ctx)
	
	return nil
}

func (h *HealthService) publishHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	heartbeatTopic := fmt.Sprintf("models.%s.heartbeat", h.config.ModelName)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := h.getHealthStatus()
			statusData, err := json.Marshal(status)
			if err != nil {
				continue
			}
			
			if err := h.nats.Publish(heartbeatTopic, statusData); err != nil {
				slog.Warn("Failed to publish heartbeat", "error", err)
			}
		}
	}
}

func (h *HealthService) getHealthStatus() HealthStatus {
	return HealthStatus{
		ModelName:    h.config.ModelName,
		Status:       "online",
		LastActivity: time.Now(),
		Capabilities: []string{"text-generation", "reasoning"},
		Endpoint:     fmt.Sprintf("http://localhost%s", h.config.HTTPAddr),
		NATSTopic:    h.config.Subject,
		Version:      "1.0.0",
	}
}