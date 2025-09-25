package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/aigoflow/inference-service/internal/config"
	"github.com/aigoflow/inference-service/internal/capabilities"
)

type HealthService struct {
	nats               *nats.Conn
	config             *config.Config
	model              capabilities.ModelInterface
	capabilityDetector capabilities.CapabilityDetector
	capabilities       []capabilities.Capability
	monitoring         *MonitoringService  // Add reference to monitoring service
	startTime          time.Time           // Track when service started
}

type HealthStatus struct {
	ModelName          string                     `json:"model_name"`
	Status             string                     `json:"status"`       // online, offline, busy
	LastActivity       time.Time                  `json:"last_activity"`
	Capabilities       []string                   `json:"capabilities"`
	Endpoint           string                     `json:"endpoint"`
	NATSTopic          string                     `json:"nats_topic"`
	Version            string                     `json:"version"`
	ModelInfo          capabilities.ModelMetadata `json:"model_info"`
	QueueMetrics       QueueMetrics               `json:"queue_metrics"`
	BackpressureStatus BackpressureStatus         `json:"backpressure_status"`
	StartTime          time.Time                  `json:"start_time"`
	Uptime             time.Duration              `json:"uptime"`
}

type QueueMetrics struct {
	PendingMessages   int64     `json:"pending_messages"`
	ActiveProcessing  int64     `json:"active_processing"`
	TotalProcessed    int64     `json:"total_processed"`
	QueueCapacity     int64     `json:"queue_capacity"`
	LastProcessedTime time.Time `json:"last_processed_time"`
}

type BackpressureStatus struct {
	Level       string  `json:"level"`        // healthy, warning, critical
	Utilization float64 `json:"utilization"`  // 0.0 to 1.0
	Threshold   int64   `json:"threshold"`    // Warning threshold
}

func NewHealthService(natsConn *nats.Conn, cfg *config.Config, model capabilities.ModelInterface, monitoring *MonitoringService) *HealthService {
	detector := capabilities.NewAutoCapabilityDetector()
	caps := detector.DetectCapabilities(model)
	startTime := time.Now()
	
	slog.Info("Health service initialized", 
		"model", cfg.ModelName,
		"capabilities", detector.GetCapabilitiesSummary(caps),
		"start_time", startTime.Format("2006-01-02 15:04:05"))
	
	return &HealthService{
		nats:               natsConn,
		config:             cfg,
		model:              model,
		capabilityDetector: detector,
		capabilities:       caps,
		monitoring:         monitoring,
		startTime:          startTime,
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
	
	// Normalize model name for NATS topic (replace dots and special chars with dashes)
	normalizedModelName := strings.ReplaceAll(h.config.ModelName, ".", "-")
	normalizedModelName = strings.ReplaceAll(normalizedModelName, "_", "-")
	heartbeatTopic := fmt.Sprintf("monitoring.models.heartbeat.%s", normalizedModelName)
	slog.Info("Starting heartbeat publishing", "model", h.config.ModelName, "normalized", normalizedModelName, "topic", heartbeatTopic)
	
	for {
		select {
		case <-ctx.Done():
			slog.Info("Heartbeat publishing stopped", "model", h.config.ModelName)
			return
		case <-ticker.C:
			// Add error recovery for getHealthStatus
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Panic in getHealthStatus", "model", h.config.ModelName, "panic", r)
					}
				}()
				
				status := h.getHealthStatus()
				statusData, err := json.Marshal(status)
				if err != nil {
					slog.Error("Failed to marshal heartbeat status", "model", h.config.ModelName, "error", err)
					return
				}
				
				if err := h.nats.Publish(heartbeatTopic, statusData); err != nil {
					slog.Error("Failed to publish heartbeat", "model", h.config.ModelName, "topic", heartbeatTopic, "error", err)
				} else {
					slog.Info("Published heartbeat", "model", h.config.ModelName, "topic", heartbeatTopic, "size", len(statusData))
				}
			}()
		}
	}
}

func (h *HealthService) getHealthStatus() HealthStatus {
	// Convert capabilities to string array for JSON
	capabilityStrings := h.capabilityDetector.GetCapabilityStrings(h.capabilities)
	
	// Safely get model metadata with null check
	var modelInfo capabilities.ModelMetadata
	if h.model != nil {
		modelInfo = h.model.GetModelMetadata()
	} else {
		slog.Error("Model interface is nil in health service", "model", h.config.ModelName)
		modelInfo = capabilities.ModelMetadata{
			Architecture: "unknown",
			Modalities:   []string{"text"},
		}
	}
	
	// Get queue metrics from monitoring service
	queueMetrics, backpressureStatus := h.getQueueMetrics()
	
	now := time.Now()
	return HealthStatus{
		ModelName:          h.config.ModelName,
		Status:             "online",
		LastActivity:       now,
		Capabilities:       capabilityStrings,
		Endpoint:           fmt.Sprintf("http://localhost%s", h.config.HTTPAddr),
		NATSTopic:          h.config.Subject,
		Version:            "1.0.0",
		ModelInfo:          modelInfo,
		QueueMetrics:       queueMetrics,
		BackpressureStatus: backpressureStatus,
		StartTime:          h.startTime,
		Uptime:             now.Sub(h.startTime),
	}
}

// getQueueMetrics retrieves current queue and backpressure metrics
func (h *HealthService) getQueueMetrics() (QueueMetrics, BackpressureStatus) {
	var queueMetrics QueueMetrics
	var backpressureStatus BackpressureStatus
	
	if h.monitoring != nil {
		// Get metrics from monitoring service
		pending := h.monitoring.GetPendingMessages()
		active := h.monitoring.GetActiveProcessing()
		total := h.monitoring.GetTotalProcessed()
		
		queueMetrics = QueueMetrics{
			PendingMessages:   pending,
			ActiveProcessing:  active,
			TotalProcessed:    total,
			QueueCapacity:     int64(h.config.MaxMsgs), // From NATS config
			LastProcessedTime: h.monitoring.GetLastProcessedTime(),
		}
		
		// Calculate backpressure status
		var utilization float64
		if h.config.MaxMsgs > 0 {
			utilization = float64(pending) / float64(h.config.MaxMsgs)
		}
		
		var level string
		threshold := int64(h.config.BackpressureThreshold)
		if threshold == 0 {
			threshold = 5 // Default threshold
		}
		
		if pending >= threshold*2 {
			level = "critical"
		} else if pending >= threshold {
			level = "warning"
		} else {
			level = "healthy"
		}
		
		backpressureStatus = BackpressureStatus{
			Level:       level,
			Utilization: utilization,
			Threshold:   threshold,
		}
	} else {
		// Default values when monitoring service is not available
		queueMetrics = QueueMetrics{
			QueueCapacity: int64(h.config.MaxMsgs),
		}
		backpressureStatus = BackpressureStatus{
			Level:     "unknown",
			Threshold: 5,
		}
	}
	
	return queueMetrics, backpressureStatus
}

// GetCapabilities returns the detected capabilities
func (h *HealthService) GetCapabilities() []capabilities.Capability {
	return h.capabilities
}

// SupportsCapability checks if the model supports a specific capability
func (h *HealthService) SupportsCapability(capability capabilities.CapabilityType) bool {
	return h.capabilityDetector.SupportsCapability(h.model, capability)
}