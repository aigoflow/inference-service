package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// ServiceStatus represents the health status of an inference service
type ServiceStatus struct {
	ModelName          string                 `json:"model_name"`
	Status             string                 `json:"status"`
	LastActivity       time.Time              `json:"last_activity"`
	Capabilities       []string               `json:"capabilities"`
	Endpoint           string                 `json:"endpoint"`
	NATSTopic          string                 `json:"nats_topic"`
	Version            string                 `json:"version"`
	ModelInfo          map[string]interface{} `json:"model_info"`
	LastSeen           time.Time              `json:"last_seen"`
	RTT                time.Duration          `json:"rtt,omitempty"`
	QueueMetrics       QueueMetrics           `json:"queue_metrics"`
	BackpressureStatus BackpressureStatus     `json:"backpressure_status"`
	FirstSeen          time.Time              `json:"first_seen"`
	Uptime             time.Duration          `json:"uptime"`
}

// QueueMetrics contains queue and processing statistics
type QueueMetrics struct {
	PendingMessages   int64     `json:"pending_messages"`
	ActiveProcessing  int64     `json:"active_processing"`
	TotalProcessed    int64     `json:"total_processed"`
	QueueCapacity     int64     `json:"queue_capacity"`
	LastProcessedTime time.Time `json:"last_processed_time"`
}

// BackpressureStatus contains backpressure health information
type BackpressureStatus struct {
	Level       string  `json:"level"`        // healthy, warning, critical
	Utilization float64 `json:"utilization"`  // 0.0 to 1.0
	Threshold   int64   `json:"threshold"`    // Warning threshold
}

// MonitorService manages inference service monitoring
type MonitorService struct {
	nats      *nats.Conn
	services  map[string]*ServiceStatus
	mu        sync.RWMutex
	listeners []chan []ServiceStatus
}

func NewMonitorService(natsURL string) (*MonitorService, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &MonitorService{
		nats:     nc,
		services: make(map[string]*ServiceStatus),
	}, nil
}

func (m *MonitorService) Start(ctx context.Context) error {
	// Subscribe to heartbeat monitoring topic
	_, err := m.nats.Subscribe("monitoring.models.heartbeat.*", func(msg *nats.Msg) {
		log.Printf("Received heartbeat on topic: %s, size: %d bytes", msg.Subject, len(msg.Data))
		
		var status ServiceStatus
		if err := json.Unmarshal(msg.Data, &status); err != nil {
			log.Printf("Failed to parse heartbeat from %s: %v", msg.Subject, err)
			log.Printf("Raw data: %s", string(msg.Data))
			return
		}
		
		now := time.Now()
		status.LastSeen = now
		
		m.mu.Lock()
		// Track first seen time for uptime calculation  
		if existing, exists := m.services[status.ModelName]; exists {
			status.FirstSeen = existing.FirstSeen // Preserve original first seen time
		} else {
			status.FirstSeen = now // First time seeing this service
		}
		status.Uptime = now.Sub(status.FirstSeen)
		
		m.services[status.ModelName] = &status
		log.Printf("Updated service status: %s -> %s (uptime: %v)", status.ModelName, status.Status, status.Uptime.Truncate(time.Second))
		m.mu.Unlock()
		
		// Notify listeners
		m.notifyListeners()
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to heartbeats: %w", err)
	}
	
	log.Println("Monitor service started, listening for heartbeats...")
	
	// Cleanup stale services every minute
	go m.cleanupStaleServices(ctx)
	
	// Proactively discover services immediately
	go m.DiscoverServices()
	
	// Periodic rediscovery every 2 minutes
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.DiscoverServices()
			}
		}
	}()
	
	return nil
}

func (m *MonitorService) cleanupStaleServices(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for name, service := range m.services {
				if now.Sub(service.LastSeen) > 2*time.Minute {
					// Mark as offline instead of deleting
					if service.Status != "offline" {
						service.Status = "offline"
						log.Printf("Marked service as offline: %s", name)
					}
				}
			}
			m.mu.Unlock()
			m.notifyListeners()
		}
	}
}

// DiscoverServices proactively discovers services via health checks
func (m *MonitorService) DiscoverServices() {
	knownModels := []string{
		"gemma3-270m", "gemma3-1b", "qwen3-4b", "gpt-oss-20b", 
		"nomic-embed-v1.5", "nomic-embed-v2-moe", "deepseek-r1-7b",
	}
	
	for _, model := range knownModels {
		go func(modelName string) {
			if status, err := m.QueryHealth(modelName); err == nil {
				m.mu.Lock()
				// Only add if not already tracked via heartbeat
				if _, exists := m.services[modelName]; !exists {
					now := time.Now()
					status.FirstSeen = now
					status.LastSeen = now
					status.Uptime = 0 // Just discovered
					m.services[modelName] = status
					log.Printf("Discovered service via health check: %s", modelName)
				}
				m.mu.Unlock()
				m.notifyListeners()
			}
		}(model)
	}
}

func (m *MonitorService) GetServices() []ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var services []ServiceStatus
	for _, service := range m.services {
		services = append(services, *service)
	}
	
	// Sort by model name
	sort.Slice(services, func(i, j int) bool {
		return services[i].ModelName < services[j].ModelName
	})
	
	return services
}

func (m *MonitorService) QueryHealth(modelName string) (*ServiceStatus, error) {
	healthTopic := fmt.Sprintf("models.%s.health", modelName)
	
	start := time.Now()
	resp, err := m.nats.Request(healthTopic, []byte("{}"), 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}
	rtt := time.Since(start)
	
	var status ServiceStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil, fmt.Errorf("failed to parse health response: %w", err)
	}
	
	status.RTT = rtt
	status.LastSeen = time.Now()
	
	return &status, nil
}

func (m *MonitorService) AddListener() chan []ServiceStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	ch := make(chan []ServiceStatus, 10)
	m.listeners = append(m.listeners, ch)
	return ch
}

func (m *MonitorService) notifyListeners() {
	services := m.GetServices()
	
	m.mu.RLock()
	for _, ch := range m.listeners {
		select {
		case ch <- services:
		default:
			// Channel full, skip
		}
	}
	m.mu.RUnlock()
}

func (m *MonitorService) Close() {
	if m.nats != nil {
		m.nats.Close()
	}
}

func main() {
	var (
		natsURL    = flag.String("nats", "nats://127.0.0.1:5700", "NATS server URL")
		httpAddr   = flag.String("http", ":5780", "HTTP server address")
		cliMode    = flag.Bool("cli", false, "Run in CLI dashboard mode")
		onceMode   = flag.Bool("once", false, "Query once and exit")
	)
	flag.Parse()

	monitor, err := NewMonitorService(*natsURL)
	if err != nil {
		log.Fatalf("Failed to create monitor service: %v", err)
	}
	defer monitor.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring
	if err := monitor.Start(ctx); err != nil {
		log.Fatalf("Failed to start monitor service: %v", err)
	}

	if *onceMode {
		// One-time status query
		time.Sleep(2 * time.Second) // Wait for initial heartbeats
		printServices(monitor.GetServices())
		return
	}

	if *cliMode {
		// CLI dashboard mode
		runCLIDashboard(ctx, monitor)
	} else {
		// HTTP server mode
		runHTTPServer(ctx, monitor, *httpAddr)
	}
}

func printServices(services []ServiceStatus) {
	if len(services) == 0 {
		fmt.Println("No inference services found")
		return
	}

	fmt.Printf("Found %d inference services:\n\n", len(services))
	
	for _, service := range services {
		fmt.Printf("🤖 %s\n", service.ModelName)
		fmt.Printf("   Architecture: %s\n", getModelArch(service))
		fmt.Printf("   Status: %s\n", service.Status)
		fmt.Printf("   Capabilities: %s\n", strings.Join(service.Capabilities, ", "))
		fmt.Printf("   Endpoint: %s\n", service.Endpoint)
		fmt.Printf("   NATS Topic: %s\n", service.NATSTopic)
		if service.RTT > 0 {
			fmt.Printf("   Response Time: %v\n", service.RTT)
		}
		fmt.Printf("   Last Seen: %v ago\n", time.Since(service.LastSeen).Truncate(time.Second))
		fmt.Println()
	}
}

func getModelArch(service ServiceStatus) string {
	if service.ModelInfo != nil {
		if arch, ok := service.ModelInfo["architecture"].(string); ok {
			return arch
		}
	}
	return "unknown"
}

func runCLIDashboard(ctx context.Context, monitor *MonitorService) {
	// Clear screen and hide cursor
	fmt.Print("\033[2J\033[H\033[?25l")
	defer fmt.Print("\033[?25h") // Show cursor on exit

	// Handle Ctrl+C gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Listen for updates
	updates := monitor.AddListener()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sigCh:
			return
		case <-ticker.C:
			// Regular refresh
			renderDashboard(monitor.GetServices())
		case <-updates:
			// Update on new data
			renderDashboard(monitor.GetServices())
		}
	}
}

func renderDashboard(services []ServiceStatus) {
	// Clear screen and move to top
	fmt.Print("\033[2J\033[H")
	
	now := time.Now()
	fmt.Printf("🔍 Inference Service Monitor - %s\n", now.Format("15:04:05"))
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")
	
	if len(services) == 0 {
		fmt.Println("❌ No inference services detected")
		fmt.Println("\n💡 Waiting for heartbeats on models.*.heartbeat...")
		return
	}
	
	fmt.Printf("📊 Active Services: %d\n\n", len(services))
	
	// Header
	fmt.Printf("%-15s %-12s %-8s %-35s %-15s %-10s\n", 
		"MODEL", "ARCH", "STATUS", "CAPABILITIES", "EMBED_DIM", "LAST_SEEN")
	fmt.Printf("%-15s %-12s %-8s %-35s %-15s %-10s\n", 
		strings.Repeat("─", 15), strings.Repeat("─", 12), strings.Repeat("─", 8), 
		strings.Repeat("─", 35), strings.Repeat("─", 15), strings.Repeat("─", 10))
	
	for _, service := range services {
		status := "🟢 " + service.Status
		if time.Since(service.LastSeen) > time.Minute {
			status = "🟡 stale"
		}
		
		arch := getModelArch(service)
		embedDim := getEmbedDim(service)
		capabilities := truncateString(strings.Join(service.Capabilities, ","), 33)
		lastSeen := formatDuration(time.Since(service.LastSeen))
		
		fmt.Printf("%-15s %-12s %-8s %-35s %-15s %-10s\n",
			service.ModelName, arch, status, capabilities, embedDim, lastSeen)
	}
	
	fmt.Printf("\n💡 Press Ctrl+C to exit\n")
}

func getEmbedDim(service ServiceStatus) string {
	if service.ModelInfo != nil {
		if size, ok := service.ModelInfo["embedding_size"].(float64); ok && size > 0 {
			return fmt.Sprintf("%dD", int(size))
		}
	}
	return "-"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func runHTTPServer(ctx context.Context, monitor *MonitorService, addr string) {
	mux := http.NewServeMux()
	
	// REST API endpoints
	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(monitor.GetServices())
	})
	
	mux.HandleFunc("/api/services/", func(w http.ResponseWriter, r *http.Request) {
		modelName := strings.TrimPrefix(r.URL.Path, "/api/services/")
		if modelName == "" {
			http.Error(w, "Model name required", http.StatusBadRequest)
			return
		}
		
		status, err := monitor.QueryHealth(modelName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(status)
	})
	
	// Server-Sent Events for real-time updates
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		// Send initial data
		services := monitor.GetServices()
		data, _ := json.Marshal(services)
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
		
		// Listen for updates
		updates := monitor.AddListener()
		defer func() {
			// Remove listener (simplified - in production would properly clean up)
		}()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.Context().Done():
				return
			case services := <-updates:
				data, _ := json.Marshal(services)
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
			}
		}
	})
	
	// HTML Dashboard
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(dashboardHTML))
	})
	
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	log.Printf("Starting HTTP monitor server on %s", addr)
	log.Printf("Dashboard: http://localhost%s", addr)
	log.Printf("API: http://localhost%s/api/services", addr)
	
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	
	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	select {
	case <-ctx.Done():
	case <-sigCh:
	}
	
	log.Println("Shutting down HTTP server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	server.Shutdown(shutdownCtx)
}

const dashboardHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Inference Service Monitor</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; margin: 20px; background: #f5f5f5; }
        .header { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .services { background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .service { padding: 15px; border-bottom: 1px solid #eee; }
        .service:last-child { border-bottom: none; }
        .service-name { font-size: 18px; font-weight: bold; color: #333; }
        .service-meta { color: #666; font-size: 14px; margin: 5px 0; }
        .capabilities { display: flex; flex-wrap: wrap; gap: 5px; margin: 8px 0; }
        .capability { background: #e3f2fd; color: #1976d2; padding: 2px 8px; border-radius: 12px; font-size: 12px; }
        .capability.tool-calling { background: #f3e5f5; color: #7b1fa2; }
        .capability.embeddings { background: #e8f5e8; color: #388e3c; }
        .status { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: bold; }
        .status.online { background: #4caf50; color: white; }
        .status.offline { background: #f44336; color: white; }
        .service.offline { opacity: 0.6; background: #f9f9f9; }
        .no-services { text-align: center; padding: 40px; color: #666; }
        .update-time { color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔍 Inference Service Monitor</h1>
        <p>Real-time monitoring of distributed AI inference services</p>
        <div class="update-time" id="lastUpdate">Connecting...</div>
    </div>
    
    <div id="services" class="services">
        <div class="no-services">🔄 Waiting for service heartbeats...</div>
    </div>

    <script>
        const servicesContainer = document.getElementById('services');
        const lastUpdateEl = document.getElementById('lastUpdate');
        
        // Connect to Server-Sent Events
        const eventSource = new EventSource('/api/events');
        
        eventSource.onmessage = function(event) {
            const services = JSON.parse(event.data);
            renderServices(services);
            lastUpdateEl.textContent = 'Last update: ' + new Date().toLocaleTimeString();
        };
        
        eventSource.onerror = function() {
            lastUpdateEl.textContent = 'Connection error - retrying...';
        };
        
        function renderServices(services) {
            if (services.length === 0) {
                servicesContainer.innerHTML = '<div class="no-services">❌ No inference services detected</div>';
                return;
            }
            
            const html = services.map(service => {
                const capabilities = service.capabilities.map(cap => {
                    const className = cap.replace(/-/g, '');
                    return '<span class="capability ' + className + '">' + cap + '</span>';
                }).join('');
                
                const embedDim = getEmbedDim(service);
                const arch = service.model_info?.architecture || 'unknown';
                const paramCount = service.model_info?.parameter_count || 'unknown';
                const lastSeen = formatLastSeen(service.last_seen);
                const queueInfo = getQueueInfo(service);
                const backpressureInfo = getBackpressureInfo(service);
                
                const serviceClass = service.status === 'offline' ? 'service offline' : 'service';
                
                return '<div class="' + serviceClass + '">' +
                    '<div class="service-name">🤖 ' + service.model_name + 
                    ' <span class="status ' + service.status + '">' + service.status + '</span></div>' +
                    '<div class="service-meta">📊 ' + arch + ' | ' + paramCount + ' parameters | ' + embedDim + '</div>' +
                    '<div class="service-meta">🌐 ' + service.endpoint + ' | 📡 ' + service.nats_topic + '</div>' +
                    '<div class="service-meta">⚡ ' + queueInfo + ' | 🎯 ' + backpressureInfo + '</div>' +
                    '<div class="capabilities">' + capabilities + '</div>' +
                    '<div class="service-meta">🕒 ' + lastSeen + ' | ⏱️ ' + getUptimeInfo(service) + '</div>' +
                    '<div class="service-meta">📅 Started: ' + getStartTimeInfo(service) + '</div>' +
                    '</div>';
            }).join('');
            
            servicesContainer.innerHTML = html;
        }
        
        function getEmbedDim(service) {
            if (service.model_info && service.model_info.embedding_size > 0) {
                return service.model_info.embedding_size + 'D embeddings';
            }
            return 'No embeddings';
        }
        
        function formatLastSeen(lastSeenStr) {
            const lastSeen = new Date(lastSeenStr);
            const now = new Date();
            const diffMs = now - lastSeen;
            const diffSec = Math.floor(diffMs / 1000);
            
            if (diffSec < 60) return diffSec + 's ago';
            if (diffSec < 3600) return Math.floor(diffSec / 60) + 'm ago';
            return Math.floor(diffSec / 3600) + 'h ago';
        }
        
        function getQueueInfo(service) {
            if (!service.queue_metrics) return 'Queue: unknown';
            
            const q = service.queue_metrics;
            const pending = q.pending_messages || 0;
            const active = q.active_processing || 0;
            const capacity = q.queue_capacity || 0;
            
            return 'Queue: ' + pending + '/' + capacity + ' pending, ' + active + ' active';
        }
        
        function getBackpressureInfo(service) {
            if (!service.backpressure_status) return 'Status: unknown';
            
            const bp = service.backpressure_status;
            const level = bp.level || 'unknown';
            const utilization = Math.round((bp.utilization || 0) * 100);
            
            return 'Backpressure: ' + level + ' (' + utilization + '%)';
        }
        
        function getUptimeInfo(service) {
            if (!service.first_seen) return 'Uptime: unknown';
            
            const firstSeen = new Date(service.first_seen);
            const now = new Date();
            const uptimeSec = Math.floor((now - firstSeen) / 1000);
            
            if (uptimeSec < 60) return 'Uptime: ' + uptimeSec + 's';
            if (uptimeSec < 3600) return 'Uptime: ' + Math.floor(uptimeSec / 60) + 'm';
            if (uptimeSec < 86400) return 'Uptime: ' + Math.floor(uptimeSec / 3600) + 'h';
            return 'Uptime: ' + Math.floor(uptimeSec / 86400) + 'd';
        }
        
        function getStartTimeInfo(service) {
            if (!service.first_seen) return 'Unknown';
            
            const firstSeen = new Date(service.first_seen);
            return firstSeen.toLocaleDateString() + ' ' + firstSeen.toLocaleTimeString();
        }
    </script>
</body>
</html>`