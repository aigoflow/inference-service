# AI Inference Service

A high-performance Go microservice for AI inference that's faster than Ollama by keeping multiple models loaded in memory and enabling true parallel execution. Built with clean architecture principles and comprehensive observability.

## 🚀 Features

- **Parallel Model Execution**: Multiple models loaded simultaneously for true parallel processing
- **Dual Access Patterns**: HTTP REST API + NATS messaging for maximum flexibility  
- **JetStream Integration**: Durable message queueing with work queue patterns for scaling
- **Grammar-Constrained Generation**: Full CRUD system for GBNF grammar management
- **Comprehensive Logging**: Request/response audit trail + operational events in SQLite
- **Metal GPU Acceleration**: Optimized for Apple Silicon with automatic GPU detection
- **Production Ready**: Clean architecture, structured logging, graceful shutdown

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

## 📋 Supported Models

| Model | Size | Port | NATS Subject | Prompt Format |
|-------|------|------|--------------|---------------|
| Gemma3-270M | 235MB | 5770 | `inference.request.gemma3-270m` | Gemma-3 |
| Qwen3-4B | 2.32GB | 5771 | `inference.request.qwen3-4b` | ChatML |
| GPT-OSS-20B | ~20GB | 5772 | `inference.request.gpt-oss-20b` | OpenAI |

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
make build-all    # Builds server + CLI tools
```

4. **Download models** (optional - models auto-download on first use)
```bash
# Models will be downloaded automatically to data/models/
# Or manually place GGUF models in:
# - data/models/gemma3-270m/model.gguf
# - data/models/qwen3-4b/model.gguf  
# - data/models/gpt-oss-20b/model.gguf
```

### Running

**Start individual models:**
```bash
make start WORKER=gemma3-270m    # Fast lightweight model
make start WORKER=qwen3-4b       # Balanced performance
make start WORKER=gpt-oss-20b    # Large high-quality model
```

**Or use shortcuts:**
```bash
make gemma3-270m    # Start Gemma
make qwen3-4b       # Start Qwen
make gpt-oss-20b    # Start GPT-OSS
```

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

## 🎯 Management Commands

```bash
# Build commands
make build-llama         # Build llama.cpp with auto-detected GPU
make build              # Build inference server
make build-cli          # Build NATS CLI client
make build-all          # Build everything

# Model management
make list-workers       # Show available model configurations
make start WORKER=name  # Start specific model
make stop WORKER=name   # Stop specific model  
make stop              # Stop all models

# Testing and logs
make test WORKER=name   # Test model endpoints
make logs WORKER=name   # View request logs
make help              # Show all commands
```

## 📊 Monitoring & Observability

### Request Logs
All inference requests are logged to SQLite with full details:
```bash
# View recent requests
curl http://localhost:5770/logs?limit=10

# Direct SQLite access
sqlite3 data/logs/gemma3-270m.sqlite \
  "SELECT req_id, input_len, tokens_out, dur_ms, status FROM requests ORDER BY id DESC LIMIT 5"
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

### Health Checks
```bash
curl http://localhost:5770/healthz  # Basic health
curl http://localhost:5770/logs     # Request history
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

**Built by Sistemica GmbH for production AI workloads requiring high performance, reliability, and scalability.**