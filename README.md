# Inference Service

A high-performance, GPU-accelerated AI inference service with NATS messaging and multi-model support.

## Features

- 🚀 **GPU Acceleration**: Metal (Apple Silicon) and CUDA support
- 🔄 **NATS Messaging**: Distributed inference via NATS JetStream
- 🤖 **Multi-Model Support**: Gemma 3, Qwen 3, GPT-OSS models
- 🧠 **Dynamic Capability Detection**: Auto-detects text, embeddings, reasoning, tool-calling
- 🔍 **Real-time Monitoring**: Web dashboard + CLI tools for service discovery
- 📦 **Client Package**: Importable Go client for easy integration
- 🔧 **Raw & Formatted Modes**: Full control over model output
- 📊 **Health Monitoring**: Real-time health checks and performance metrics
- 🎯 **Production Ready**: Proper error handling, timeouts, logging

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │    │   NATS Client   │
└─────────┬───────┘    └─────────┬───────┘
          │                      │
          ▼                      ▼
┌─────────────────────────────────────────┐
│           HTTP Handler                  │
│          NATS Handler                   │
└─────────────┬───────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│        Inference Service                │
│     (Business Logic Layer)              │
└─────────────┬───────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│        Repository Layer                 │
│     (Data Access Interface)             │
└─────────────┬───────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│          SQLite Database                │
│    (Audit Logs + Event Tracking)       │
└─────────────────────────────────────────┘
```

## 🧪 Tested Models

### Text Generation Models

| Model | Size | Port | NATS Subject | Prompt Format |
|-------|------|------|--------------|---------------|
| Gemma3-270M | 235MB | 5770 | `inference.request.gemma3-270m` | Gemma-3 |
| Qwen3-4B | 2.32GB | 5771 | `inference.request.qwen3-4b` | ChatML |
| GPT-OSS-20B | ~20GB | 5772 | `inference.request.gpt-oss-20b` | OpenAI |

### Embedding Models

| Model | Size | Port | NATS Subject | Languages | Dimensions |
|-------|------|------|--------------|-----------|------------|
| nomic-embed-v1.5 | 261MB | 5778 | `embedding.request.nomic-embed-v1.5` | English | 768 |
| nomic-embed-v2-moe | 913MB | 5779 | `embedding.request.nomic-embed-v2-moe` | 101 languages | 768 |

*All models are automatically downloaded on first use if not present.*

## 🛠️ Quick Start

### Prerequisites

- Go 1.21+
- NATS Server (for messaging)
- macOS (Metal GPU) or Linux
- ~8GB RAM minimum for smaller models

### Installation

1. **Clone the repository**
```bash
git clone <repository-url>
cd inference-service
```

2. **Build llama.cpp with GPU acceleration**
```bash
make build-llama  # Auto-detects best GPU support
```

3. **Build the inference server**
```bash
make build-all    # Builds server + CLI tools (nats-chat + nats-embed)
```

### Running (via Makefile)

All operations use the Makefile for consistency. Models auto-download on first use:

**Start individual models:**
```bash
make start WORKER=gemma3-270m    # Fast 235MB model (auto-downloads)
make start WORKER=qwen3-4b       # Balanced 2.3GB model (auto-downloads)
make start WORKER=gpt-oss-20b    # Large 20GB model (auto-downloads)
```

**Or use shortcuts:**
```bash
make gemma3-270m    # Same as: make start WORKER=gemma3-270m
make qwen3-4b       # Same as: make start WORKER=qwen3-4b  
make gpt-oss-20b    # Same as: make start WORKER=gpt-oss-20b
```

**First run:** Models download automatically with progress logging
**Subsequent runs:** Start immediately (models cached locally)

## 🔧 Usage

### HTTP API

**Basic inference:**
```bash
curl -X POST http://localhost:5770/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Explain machine learning",
    "params": {
      "temperature": 0.7
    }
  }'
```

**With parameters:**
```bash  
curl -X POST http://localhost:5770/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Write a function to sort an array",
    "params": {
      "max_tokens": 500,
      "temperature": 0.3,
      "grammar": "code"
    }
  }'
```

### NATS Messaging

**One-shot inference:**
```bash
./bin/nats-chat -subject inference.request.qwen3-4b -input "What is quantum computing?"
```

**Interactive chat:**
```bash
./bin/nats-chat -subject inference.request.gemma3-270m -i
```

**Custom parameters:**
```bash
./bin/nats-chat \
  -subject inference.request.gpt-oss-20b \
  -input "Explain neural networks" \
  -max-tokens 1000 \
  -temperature 0.1
```

**Raw NATS Messages:**
```bash
# Direct NATS request (formatted)
echo '{
  "req_id": "manual-req-123",
  "input": "Hello, world!",
  "params": {
    "max_tokens": 50,
    "temperature": 0.7
  }
}' | nats req inference.request.gemma3-270m --timeout=30s

# Direct NATS request (raw mode) 
echo '{
  "req_id": "manual-raw-456", 
  "input": "Hello",
  "params": {"max_tokens": 20},
  "raw": true
}' | nats req inference.request.qwen3-4b --timeout=30s

# Response format:
{
  "req_id": "manual-req-123",
  "text": "Hello! How can I help you today?",
  "tokens_in": 12,
  "tokens_out": 8,
  "finish_reason": "stop",
  "duration_ms": 156
}
```

## 📊 Embeddings

### HTTP Embeddings API

**Basic embedding:**
```bash
curl -X POST http://localhost:5778/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "search_query: machine learning",
    "model": "nomic-embed-v1.5"
  }'
```

**Multilingual embedding:**
```bash
curl -X POST http://localhost:5779/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Hola mundo",
    "model": "nomic-embed-v2-moe"
  }'
```

**Multiple inputs:**
```bash
curl -X POST http://localhost:5778/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "input": ["Hello world", "Machine learning", "Artificial intelligence"],
    "model": "nomic-embed-v1.5"
  }'
```

### NATS Embedding Messaging

**English embedding:**
```bash
./bin/nats-embed -input "search_query: machine learning"
```

**Multilingual embedding:**
```bash
./bin/nats-embed -subject embedding.request.nomic-embed-v2-moe -input "Bonjour le monde" -model nomic-embed-v2-moe
```

**Raw NATS embedding requests:**
```bash
echo '{
  "req_id": "embed-123",
  "input": "Hello world",
  "model": "nomic-embed-v1.5",
  "reply_to": "embed.reply.test"
}' | nats req embedding.request.nomic-embed-v1.5 --server=nats://127.0.0.1:5700 --timeout=10s
```

**Response format:**
```json
{
  "object": "list",
  "data": [{
    "object": "embedding",
    "embedding": [-0.006747, -0.001334, -0.171558, ...],
    "index": 0
  }],
  "model": "nomic-embed-v1.5",
  "usage": {
    "prompt_tokens": 4,
    "total_tokens": 4
  }
}
```

### Grammar Management

**Create grammar:**
```bash
curl -X POST http://localhost:5770/grammars \
  -H "Content-Type: application/json" \
  -d '{
    "name": "json-schema",
    "description": "JSON object validator", 
    "content": "root ::= object\nobject ::= \"{\" ws \"}\" ..."
  }'
```

**List grammars:**
```bash
curl http://localhost:5770/grammars
```

**Use grammar in inference:**
```bash
curl -X POST http://localhost:5770/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Generate a JSON user profile",
    "params": {
      "grammar": "json-schema",
      "temperature": 0.1
    }
  }'
```

## 🎯 Makefile Commands

All operations are managed through the Makefile for consistency and ease of use:

### Build Commands
```bash
make build-llama         # Build llama.cpp with auto-detected GPU
make build              # Build inference server
make build-cli          # Build NATS CLI client
make build-monitor      # Build monitoring tool
make build-all          # Build everything
```

### Model Management
```bash
make list-workers       # Show available model configurations
make start WORKER=name  # Start specific model (auto-downloads if missing)
make stop WORKER=name   # Stop specific model  
make stop              # Stop all models
```

### Testing and Monitoring
```bash
make test WORKER=name   # Test model endpoints
make logs WORKER=name   # View request logs from SQLite
make monitor-status     # Get current service status
make monitor-cli        # Start CLI monitoring dashboard
make monitor-dashboard  # Start web dashboard (http://localhost:8080)
make help              # Show all available commands
```

### Shortcuts (equivalent to make start WORKER=name)
```bash
# Text Generation Models
make gemma3-270m        # Start Gemma3-270M
make qwen3-4b          # Start Qwen3-4B  
make gpt-oss-20b       # Start GPT-OSS-20B

# Embedding Models
make nomic-embed-v1.5  # Start English embedding model
make nomic-embed-v2-moe # Start multilingual MoE embedding model
```

**Note**: Models are automatically downloaded with progress indication on first use.

## 🔍 Service Monitoring & Discovery

### Real-time Monitoring Dashboard

**Web Dashboard (Recommended):**
```bash
make monitor-dashboard
# Opens: http://localhost:8080
```

Features:
- 🌐 **Real-time updates** via Server-Sent Events
- 📊 **Capability matrix** showing detected features per model
- 🎯 **Architecture detection** (gemma3, qwen3, gpt-oss, nomic-bert)
- 📈 **Parameter counts** (268M, 4.0B, 20.9B)
- 🔗 **Endpoint mapping** (HTTP + NATS topics)
- ⚡ **Response times** and last seen timestamps

**CLI Dashboard (htop-style):**
```bash
make monitor-cli
# Real-time terminal dashboard with auto-refresh
```

**One-time Status:**
```bash
make monitor-status
# Quick snapshot of all running services
```

### REST API for Integration

**List all services:**
```bash
curl http://localhost:5780/api/services | jq
```

**Query specific service:**
```bash
curl http://localhost:5780/api/services/gemma3-270m | jq
```

**Real-time events stream:**
```bash
curl -N http://localhost:5780/api/events
# Server-Sent Events stream for real-time updates
```

### Enhanced Features

**Complete Service Information:**
- 🧠 **Dynamic Capability Detection**: Auto-detects text, embeddings, reasoning, tool-calling
- 📊 **Queue Metrics**: Pending messages, active processing, queue capacity  
- 🎯 **Backpressure Status**: Health levels (healthy/warning/critical) with utilization %
- ⏱️ **Uptime Tracking**: Time since monitor first discovered each service
- 📅 **Discovery Timestamps**: Exact time when services were first seen
- 🔄 **Real-time Updates**: Live data via NATS heartbeat system

**API Response Example:**
```json
{
  "model_name": "qwen3-4b",
  "status": "online",
  "capabilities": ["text-generation", "embeddings", "reasoning", "tool-calling"],
  "model_info": {
    "architecture": "qwen3",
    "embedding_size": 2560,
    "parameter_count": "4.0B"
  },
  "queue_metrics": {
    "pending_messages": 0,
    "active_processing": 0,
    "queue_capacity": 2000
  },
  "backpressure_status": {
    "level": "healthy",
    "utilization": 0.0
  },
  "first_seen": "2025-09-25T15:12:48+02:00",
  "uptime": 300000000000
}
```

## 📊 Monitoring & Observability

### Model Discovery & Health Checks

**NATS Health Checks:**
```bash
# Check text generation model health
echo '{}' | nats req models.gemma3-270m.health --server=nats://127.0.0.1:5700 --timeout=5s
echo '{}' | nats req models.qwen3-4b.health --server=nats://127.0.0.1:5700 --timeout=5s  
echo '{}' | nats req models.gpt-oss-20b.health --server=nats://127.0.0.1:5700 --timeout=5s

# Check embedding model health
echo '{}' | nats req models.nomic-embed-v1.5.health --server=nats://127.0.0.1:5700 --timeout=5s
echo '{}' | nats req models.nomic-embed-v2-moe.health --server=nats://127.0.0.1:5700 --timeout=5s

# Enhanced response format with dynamic capability detection:
{
  "model_name": "gemma3-270m",
  "status": "online",
  "last_activity": "2025-09-25T10:30:15Z",
  "capabilities": ["text-generation", "embeddings", "reasoning", "grammar-constrained"],
  "endpoint": "http://localhost:5770",
  "nats_topic": "inference.request.gemma3-270m",
  "version": "1.0.0",
  "model_info": {
    "architecture": "gemma3",
    "modalities": ["text", "embeddings"],
    "parameter_count": "268M",
    "context_size": 8192,
    "embedding_size": 640,
    "quantization": "Q4_K_M",
    "model_family": "gemma"
  }
}
```

**HTTP Health Checks:**
```bash
curl http://localhost:5770/healthz  # Basic health
curl http://localhost:5771/healthz  # Qwen health
curl http://localhost:5772/healthz  # GPT-OSS health
```

### Real-time Monitoring

**Backpressure Monitoring:**
```bash
# Monitor all models
nats sub "monitoring.inference.*"

# Monitor specific model
nats sub "monitoring.inference.gemma3-270m"

# Example monitoring output:
{
  "model_name": "gemma3-270m",
  "pending_messages": 3,
  "active_processing": 2, 
  "timestamp": "2025-09-23T18:30:15Z",
  "worker_count": 2,
  "queue_capacity": 2000,
  "status": "warning"
}
```

**Monitoring Frequencies:**
- **1 second**: When pending_messages > 0 (under load)
- **10 seconds**: When pending_messages = 0 (idle)
- **Status levels**: healthy, warning, critical

**Heartbeat Monitoring:**
```bash
# Monitor model heartbeats
nats sub "models.*.heartbeat"

# Monitor specific model heartbeats  
nats sub "models.gemma3-270m.heartbeat"
```

### Request Logs
All inference requests are logged to SQLite with full details:
```bash
# View recent requests with worker IDs
sqlite3 data/logs/gemma3-270m.sqlite \
  "SELECT req_id, worker_id, raw_input, formatted_input, dur_ms, status, datetime(ts, 'unixepoch') as time FROM requests ORDER BY ts DESC LIMIT 5"

# View requests by worker
sqlite3 data/logs/gemma3-270m.sqlite \
  "SELECT worker_id, COUNT(*) as request_count, AVG(dur_ms) as avg_latency FROM requests GROUP BY worker_id"

# Monitor backpressure trends
sqlite3 data/logs/gemma3-270m.sqlite \
  "SELECT datetime(ts, 'unixepoch') as time, dur_ms, tokens_out FROM requests ORDER BY ts DESC LIMIT 20"
```

### Event Logs  
Operational events tracked in SQLite:
```bash
sqlite3 data/logs/gemma3-270m.sqlite \
  "SELECT datetime(ts, 'unixepoch') as time, level, code, msg FROM events ORDER BY ts DESC LIMIT 10"
```

Events include:
- `startup` - Server initialization
- `model.loading` - Model loading started  
- `model.loaded` - Model ready for inference
- `services.init` - HTTP/NATS services starting
- `server.ready` - Ready to accept requests
- `health.started` - Health monitoring active
- `monitoring.started` - Backpressure monitoring active

## 🔧 Advanced Features

### Raw Mode Control

**HTTP Raw Mode:**
```bash
# Bypass all formatting - pure model output
curl -X POST http://localhost:5770/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "input": "Hello",
    "params": {"max_tokens": 20},
    "raw": true
  }'

# Raw response (no template formatting):
{
  "ms": 150,
  "text": " there!\n\nI hope you're doing well",
  "tokens_in": 1,
  "tokens_out": 10
}
```

**NATS Raw Mode:**
```bash
# Raw mode via NATS messaging
echo '{
  "req_id": "test-raw-123",
  "input": "Complete: The capital of France is",
  "params": {"max_tokens": 5},
  "raw": true
}' | nats req inference.request.gemma3-270m --timeout=30s
```

### Model Format Configuration

**Template Format (Default for Gemma/Qwen):**
```bash
# Environment: MODEL_FORMAT=template
# Uses: data/models/{model}/prompt_template.json

# Example template file:
{
  "name": "Gemma-3",
  "user_prefix": "<start_of_turn>user\n",
  "user_suffix": "<end_of_turn>\n",
  "model_prefix": "<start_of_turn>model\n",
  "model_suffix": ""
}
```

**Harmony Format (GPT-OSS Models):**
```bash
# Environment: MODEL_FORMAT=harmony
# Uses: OpenAI Harmony response format

# Harmony configuration:
HARMONY_REASONING_LEVEL=medium
HARMONY_EXTRACT_FINAL=true
HARMONY_MODEL_IDENTITY=ChatGPT, a large language model trained by OpenAI
HARMONY_KNOWLEDGE_CUTOFF=2024-06
```

### Service Discovery & Integration

**Model Discovery:**
```bash
# Discover all available models
for model in gemma3-270m qwen3-4b gpt-oss-20b; do
  echo "Checking $model..."
  echo '{}' | nats req models.$model.health --timeout=2s | jq -r '.status'
done

# Output:
# Checking gemma3-270m...
# online
# Checking qwen3-4b...  
# online
# Checking gpt-oss-20b...
# online
```

**Load Balancing Information:**
```bash
# Monitor all model loads
nats sub "monitoring.inference.*" | jq '.model_name, .pending_messages, .status'

# Choose least loaded model
echo '{}' | nats req models.gemma3-270m.health | jq -r '.nats_topic'
# Output: inference.request.gemma3-270m
```

### Multi-Model Workflows

**Model Selection Strategy:**
```bash
# Fast responses: Use Gemma3-270M
curl -X POST http://localhost:5770/v1/completions \
  -d '{"input": "Quick math: 2+2", "params": {"max_tokens": 10}}'

# Balanced reasoning: Use Qwen3-4B  
curl -X POST http://localhost:5771/v1/completions \
  -d '{"input": "Explain photosynthesis", "params": {"max_tokens": 200}}'

# Advanced reasoning: Use GPT-OSS-20B
curl -X POST http://localhost:5772/v1/completions \
  -d '{"input": "Design a multi-agent system architecture", "params": {"max_tokens": 500}}'
```

## 🧪 Testing

### Automated Test Suites

**Grammar CRUD testing:**
```bash
./examples/test-grammar-crud.sh
```

**Inference endpoints testing:**
```bash  
./examples/test-inference-endpoints.sh
```

**Parallel processing verification:**
```bash
./examples/test-parallel.sh
```

### Manual Testing

**Test all models:**
```bash
# Start all models
make gemma3-270m &
make qwen3-4b &  
make gpt-oss-20b &

# Test each model
make test WORKER=gemma3-270m
make test WORKER=qwen3-4b
make test WORKER=gpt-oss-20b
```

## 🔧 Configuration

### Environment Variables

Models are configured via `.env` files in `envs/worker.{model}.env`:

```bash
# NATS Configuration
NATS_URL=nats://127.0.0.1:4222
SUBJECT=inference.request.model-name
WORKER_CONCURRENCY=2

# HTTP Configuration  
HTTP_ADDR=:5770

# Model Configuration
MODEL_NAME=model-name
MODEL_PATH=data/models/model-name/model.gguf
MODEL_THREADS=8
CTX_SIZE=8192

# Database
DB_PATH=data/logs/model-name.sqlite
```

### Custom Models

To add a new model:

1. **Create env file:** `envs/worker.{name}.env`
2. **Add model file:** `data/models/{name}/model.gguf`  
3. **Create prompt template:** `data/models/{name}/prompt_template.json`
4. **Start model:** `make start WORKER={name}`

Example prompt template:
```json
{
  "name": "Model-Name",
  "user_prefix": "<|user|>\n",
  "user_suffix": "\n",
  "model_prefix": "<|assistant|>\n", 
  "model_suffix": ""
}
```

## 🏗️ Development

### Project Structure

```
├── cmd/server/          # Main application entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── handlers/       # HTTP request handlers  
│   ├── services/       # Business logic (inference, NATS, grammar)
│   ├── repository/     # Data access layer
│   ├── models/         # Domain models and DTOs
│   ├── llama/          # llama.cpp CGO integration
│   └── store/          # SQLite database layer
├── examples/           # CLI tools and test scripts
├── envs/              # Model configuration files
├── data/              # Models and databases
└── scripts/           # Build and utility scripts
```

### Architecture Principles

- **Layered Architecture**: Handler → Service → Repository → Database
- **Dependency Injection**: Interfaces for all external dependencies
- **Clean Separation**: No business logic in handlers, no database access in services
- **Repository Pattern**: Abstract data access behind interfaces
- **Structured Logging**: All operations logged with context

### Building from Source

```bash
# Install dependencies
go mod download

# Build llama.cpp
make build-llama-metal   # macOS with Metal
make build-llama-cuda    # NVIDIA GPU  
make build-llama-cpu     # CPU only

# Build application
make build

# Run tests
go test ./...
```

## 📈 Performance

### Benchmarks

**Model Loading Times:**
- Gemma3-270M: ~3 seconds
- Qwen3-4B: ~5 seconds  
- GPT-OSS-20B: ~15 seconds

**Inference Performance:**
- Gemma3-270M: ~100ms average
- Qwen3-4B: ~2000ms average
- GPT-OSS-20B: ~5000ms average

**Parallel Processing:**
- Up to 2x speedup with concurrent requests
- JetStream queueing prevents message loss
- Independent model execution (no cross-blocking)

### Memory Usage

- Base system: ~100MB
- + Gemma3-270M: ~300MB  
- + Qwen3-4B: ~2.5GB
- + GPT-OSS-20B: ~20GB

## 🔐 Production Deployment

### Docker Support

```bash
# Build with GPU support
docker build -t inference-service .

# Run with GPU
docker run --gpus all -p 5770:5770 -v ./data:/app/data inference-service
```

### Monitoring

- **Prometheus metrics**: `/metrics` endpoint
- **Health checks**: `/healthz` endpoint  
- **Request tracing**: Full audit trail in SQLite
- **Structured logging**: JSON format for log aggregation

### Scaling

- **Horizontal**: Run multiple instances with different models
- **Vertical**: Increase `WORKER_CONCURRENCY` per model
- **Load Balancing**: Use NATS subjects for request distribution

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Follow the architecture patterns in `CLAUDE.md`
4. Add tests for new functionality
5. Submit a pull request

### Code Style

- Follow Go conventions and `gofmt`
- Use structured logging with `slog`
- Maintain layered architecture  
- Add comprehensive tests
- Document public APIs

## 📄 License

MIT License

Copyright (c) 2025 Sistemica GmbH

See LICENSE file for full details.

## 🙏 Acknowledgments

This project builds upon excellent open source projects:

- **[llama.cpp](https://github.com/ggerganov/llama.cpp)** - High-performance LLM inference engine (MIT License)
- **[NATS](https://github.com/nats-io/nats-server)** - Cloud native messaging system (Apache 2.0)
- **[Go](https://golang.org/)** - Programming language and runtime (BSD-style)
- **[SQLite](https://sqlite.org/)** - Embedded database (Public Domain)

Model providers:
- **[Google](https://huggingface.co/google)** - Gemma3 model family
- **[Alibaba](https://huggingface.co/Qwen)** - Qwen3 model family  
- **[OpenAI](https://huggingface.co/openai)** - GPT-OSS model family

## 🔗 Related Projects

- [Ollama](https://ollama.ai/) - Alternative LLM serving platform
- [vLLM](https://github.com/vllm-project/vllm) - High-throughput LLM serving
- [Text Generation Inference](https://github.com/huggingface/text-generation-inference) - Hugging Face inference server

---

## 📡 NATS Topics Reference

### Model Communication Topics

**Inference Requests:**
```bash
# Send inference requests
inference.request.gemma3-270m     # Fast lightweight model
inference.request.qwen3-4b        # Balanced performance model  
inference.request.gpt-oss-20b     # Advanced reasoning model
```

**Health & Discovery:**
```bash
# Health check requests (request/reply)
models.gemma3-270m.health         # Check Gemma health
models.qwen3-4b.health           # Check Qwen health
models.gpt-oss-20b.health        # Check GPT-OSS health

# Heartbeat broadcasts (publish only)
models.gemma3-270m.heartbeat     # Gemma heartbeats (every 30s)
models.qwen3-4b.heartbeat        # Qwen heartbeats (every 30s) 
models.gpt-oss-20b.heartbeat     # GPT-OSS heartbeats (every 30s)
```

**Monitoring & Observability:**
```bash
# Backpressure monitoring (publish only)
monitoring.inference.gemma3-270m  # Gemma load reports
monitoring.inference.qwen3-4b     # Qwen load reports
monitoring.inference.gpt-oss-20b  # GPT-OSS load reports

# Aggregate monitoring (future)
monitoring.inference.*            # All model reports
monitoring.aggregate              # System-wide metrics
monitoring.alerts                 # Critical alerts
```

### Example NATS Message Flows

**Health Check Flow:**
```bash
# 1. Send health request
echo '{}' | nats req models.gemma3-270m.health --timeout=5s

# 2. Receive health response
{
  "model_name": "gemma3-270m",
  "status": "online",
  "capabilities": ["text-generation", "reasoning"],
  "endpoint": "http://localhost:5770",
  "nats_topic": "inference.request.gemma3-270m"
}
```

**Inference Request Flow:**
```bash
# 1. Send inference request
echo '{
  "req_id": "test-123",
  "input": "What is machine learning?",
  "params": {"max_tokens": 100, "temperature": 0.7}
}' | nats req inference.request.qwen3-4b --timeout=30s

# 2. Receive inference response
{
  "req_id": "test-123",
  "text": "Machine learning is a subset of artificial intelligence...",
  "tokens_in": 15,
  "tokens_out": 87,
  "finish_reason": "stop",
  "duration_ms": 1250
}
```

**Monitoring Subscription:**
```bash
# Subscribe to all monitoring data
nats sub "monitoring.inference.*"

# Real-time monitoring output:
[#1] Received on "monitoring.inference.gemma3-270m"
{"model_name":"gemma3-270m","pending_messages":0,"active_processing":0,"status":"healthy"}

[#2] Received on "monitoring.inference.qwen3-4b"  
{"model_name":"qwen3-4b","pending_messages":2,"active_processing":1,"status":"warning"}
```

## 🔌 Integration Patterns

### For Reasoning Services
```bash
# 1. Check model availability
echo '{}' | nats req models.qwen3-4b.health

# 2. Monitor current load  
nats sub "monitoring.inference.qwen3-4b" --count=1

# 3. Send reasoning request with raw control
echo '{
  "req_id": "reasoning-456",
  "input": "<|start|>system<|message|>You are an expert...",
  "params": {"max_tokens": 500},
  "raw": true
}' | nats req inference.request.gpt-oss-20b
```

### For Agent Services
```bash
# 1. Discover available models
for model in gemma3-270m qwen3-4b gpt-oss-20b; do
  status=$(echo '{}' | nats req models.$model.health --timeout=2s | jq -r '.status')
  echo "$model: $status"
done

# 2. Choose model based on load
best_model=$(nats sub "monitoring.inference.*" --count=3 | \
  jq -r 'select(.status=="healthy") | .model_name' | head -1)

# 3. Route request to best model  
echo "Routing to: $best_model"
```

---

**Built by Sistemica GmbH for production AI workloads requiring high performance, reliability, and scalability.**