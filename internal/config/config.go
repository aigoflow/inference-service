package config

import (
	"bufio"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// NATS Configuration
	NatsURL         string
	Stream          string
	Subject         string
	Durable         string
	QueueGroup      string
	ResponsePrefix  string
	MaxMsgs         int
	MaxAge          time.Duration
	AckWait         time.Duration
	MaxDeliver      int
	MaxAckPending   int
	Concurrency     int
	
	// HTTP Configuration
	HTTPAddr string
	
	// Model Configuration
	ModelName string
	ModelURL  string
	ModelPath string
	Threads   int
	CtxSize   int
	
	// Data Directory Configuration
	DataDir string
	
	// Database Configuration
	DBPath string
}

func Load(envFile string) (*Config, error) {
	if envFile != "" {
		if err := loadDotEnv(envFile); err != nil {
			slog.Warn("Could not load env file", "file", envFile, "error", err)
		} else {
			slog.Info("Environment loaded", "file", envFile)
		}
	}

	return &Config{
		NatsURL:        getEnv("NATS_URL", "nats://127.0.0.1:4222"),
		Stream:         getEnv("STREAM_NAME", "INFER"),
		Subject:        getEnv("SUBJECT", "inference.request.default"),
		Durable:        getEnv("QUEUE_DURABLE", "infer-wq"),
		QueueGroup:     getEnv("QUEUE_GROUP", "workers"),
		ResponsePrefix: getEnv("RESPONSE_PREFIX", "inference.reply"),
		MaxMsgs:        getEnvInt("QUEUE_MAX_MSGS", 2000),
		MaxAge:         getEnvDuration("QUEUE_MAX_AGE", "30s"),
		AckWait:        getEnvDuration("ACK_WAIT", "30s"),
		MaxDeliver:     getEnvInt("MAX_DELIVER", 5),
		MaxAckPending:  getEnvInt("MAX_ACK_PENDING", 64),
		Concurrency:    getEnvInt("WORKER_CONCURRENCY", 2),
		HTTPAddr:       getEnv("HTTP_ADDR", ":8081"),
		ModelName:      getEnv("MODEL_NAME", "default"),
		ModelURL:       getEnv("MODEL_URL", ""),
		ModelPath:      getEnv("MODEL_PATH", "data/models/model.gguf"),
		Threads:        getEnvInt("MODEL_THREADS", 8),
		CtxSize:        getEnvInt("CTX_SIZE", 4096),
		DataDir:        getEnv("DATA_DIR", "data"),
		DBPath:         getEnv("DB_PATH", "data/worker.sqlite"),
	}, nil
}

func loadDotEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key, defaultVal string) time.Duration {
	val := getEnv(key, defaultVal)
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}
	d, _ := time.ParseDuration(defaultVal)
	return d
}