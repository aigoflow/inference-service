package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/aigoflow/inference-service/internal/config"
)

type MonitoringService struct {
	nats         *nats.Conn
	config       *config.Config
	pendingCount int64 // atomic counter
	activeCount  int64 // atomic counter for active processing
}

type BackpressureReport struct {
	ModelName        string    `json:"model_name"`
	PendingMessages  int64     `json:"pending_messages"`
	ActiveProcessing int64     `json:"active_processing"`
	Timestamp        time.Time `json:"timestamp"`
	WorkerCount      int       `json:"worker_count"`
	QueueCapacity    int       `json:"queue_capacity"`
	Status           string    `json:"status"` // healthy, warning, critical
}

func NewMonitoringService(natsConn *nats.Conn, cfg *config.Config) *MonitoringService {
	return &MonitoringService{
		nats:   natsConn,
		config: cfg,
	}
}

func (m *MonitoringService) Start(ctx context.Context) error {
	slog.Info("Starting monitoring service", 
		"topic", m.config.MonitoringTopic,
		"threshold", m.config.BackpressureThreshold)
	
	// Start backpressure monitoring
	go m.monitorBackpressure(ctx)
	
	return nil
}

func (m *MonitoringService) monitorBackpressure(ctx context.Context) {
	// Different intervals based on load
	highLoadTicker := time.NewTicker(1 * time.Second)  // When pending > 0
	lowLoadTicker := time.NewTicker(10 * time.Second)  // When pending = 0
	defer highLoadTicker.Stop()
	defer lowLoadTicker.Stop()
	
	var currentTicker *time.Ticker = lowLoadTicker
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-currentTicker.C:
			pending := atomic.LoadInt64(&m.pendingCount)
			active := atomic.LoadInt64(&m.activeCount)
			
			// Switch ticker based on load
			if pending > 0 && currentTicker == lowLoadTicker {
				currentTicker = highLoadTicker
				slog.Debug("Switched to high-frequency monitoring", "pending", pending)
			} else if pending == 0 && currentTicker == highLoadTicker {
				currentTicker = lowLoadTicker
				slog.Debug("Switched to low-frequency monitoring")
			}
			
			// Always report, but frequency depends on load
			m.reportBackpressure(pending, active)
		}
	}
}

func (m *MonitoringService) reportBackpressure(pending, active int64) {
	status := m.calculateStatus(pending, active)
	
	report := BackpressureReport{
		ModelName:        m.config.ModelName,
		PendingMessages:  pending,
		ActiveProcessing: active,
		Timestamp:        time.Now(),
		WorkerCount:      m.config.Concurrency,
		QueueCapacity:    int(m.config.MaxMsgs),
		Status:           status,
	}
	
	reportData, err := json.Marshal(report)
	if err != nil {
		slog.Error("Failed to marshal backpressure report", "error", err)
		return
	}
	
	topic := fmt.Sprintf("%s.%s", m.config.MonitoringTopic, m.config.ModelName)
	if err := m.nats.Publish(topic, reportData); err != nil {
		slog.Warn("Failed to publish backpressure report", "error", err)
		return
	}
	
	// Log significant changes
	if pending > 0 || status != "healthy" {
		slog.Info("Backpressure report", 
			"pending", pending,
			"active", active,
			"status", status)
	}
}

func (m *MonitoringService) calculateStatus(pending, active int64) string {
	total := pending + active
	threshold := int64(m.config.BackpressureThreshold)
	
	if total == 0 {
		return "healthy"
	} else if total < threshold {
		return "warning"
	} else {
		return "critical"
	}
}

// IncrementPending atomically increments pending message count
func (m *MonitoringService) IncrementPending() {
	atomic.AddInt64(&m.pendingCount, 1)
}

// DecrementPending atomically decrements pending message count
func (m *MonitoringService) DecrementPending() {
	atomic.AddInt64(&m.pendingCount, -1)
}

// IncrementActive atomically increments active processing count
func (m *MonitoringService) IncrementActive() {
	atomic.AddInt64(&m.activeCount, 1)
}

// DecrementActive atomically decrements active processing count
func (m *MonitoringService) DecrementActive() {
	atomic.AddInt64(&m.activeCount, -1)
}

// GetPendingCount returns current pending count
func (m *MonitoringService) GetPendingCount() int64 {
	return atomic.LoadInt64(&m.pendingCount)
}

// GetActiveCount returns current active count
func (m *MonitoringService) GetActiveCount() int64 {
	return atomic.LoadInt64(&m.activeCount)
}