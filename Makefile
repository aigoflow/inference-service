.PHONY: build clean list-workers stop help build-deps build-llama build-native build-metal

# Auto-detect available worker configurations
WORKER_ENVS := $(wildcard envs/worker.*.env)
WORKER_NAMES := $(patsubst envs/worker.%.env,%,$(WORKER_ENVS))

# Auto-detect and build llama.cpp with best available acceleration
build-llama:
	@GPU_TYPE=$$(./scripts/detect-gpu.sh | tail -1) && \
	echo "Auto-detected GPU type: $$GPU_TYPE" && \
	./scripts/build-llama.sh $$GPU_TYPE

# Build with specific acceleration (cpu|metal|cuda|rocm|vulkan)
build-llama-%:
	./scripts/build-llama.sh $*

# Build C++ binding only if source files changed
internal/llama/libbinding.a: internal/llama/binding.cpp internal/llama/binding.h
	@echo "Building C++ binding..."
	cd internal/llama && \
	c++ -O3 -DNDEBUG -std=c++11 -fPIC -c binding.cpp -I./include -I./src -I./ggml_include && \
	ar rcs libbinding.a binding.o

# Build the server binary  
bin/inference-server: internal/llama/libbinding.a internal/llama/libllama.a $(shell find . -name "*.go" -not -path "./examples/*")
	@echo "Building inference server..."
	CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" go build -o bin/inference-server ./cmd/server

build: bin/inference-server

# Build the NATS CLI client
build-cli:
	@echo "Building NATS CLI client..."
	go build -o bin/nats-chat ./examples/nats-chat.go

# Build everything
build-all: build build-cli


# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f bin/inference-server
	rm -rf data/logs/*.sqlite

# List available workers
list-workers:
	@echo "Available workers:"
	@for worker in $(WORKER_NAMES); do \
		port=$$(grep "HTTP_ADDR=" envs/worker.$$worker.env | cut -d':' -f2); \
		model=$$(grep "MODEL_NAME=" envs/worker.$$worker.env | cut -d'=' -f2); \
		echo "  $$worker ($$model) - port $$port"; \
	done

# Generic worker starter - usage: make start WORKER=gemma3-270m
start: build
	@if [ -z "$(WORKER)" ]; then \
		echo "Usage: make start WORKER=<name>"; \
		echo "Available workers: $(WORKER_NAMES)"; \
		exit 1; \
	fi
	@if [ ! -f "envs/worker.$(WORKER).env" ]; then \
		echo "Error: envs/worker.$(WORKER).env not found"; \
		echo "Available workers: $(WORKER_NAMES)"; \
		exit 1; \
	fi
	@echo "Starting server: $(WORKER)"
	@port=$$(grep "HTTP_ADDR=" envs/worker.$(WORKER).env | cut -d':' -f2); \
	model=$$(grep "MODEL_NAME=" envs/worker.$(WORKER).env | cut -d'=' -f2); \
	echo "Model: $$model, Port: $$port"; \
	./bin/inference-server -env envs/worker.$(WORKER).env

# Stop specific worker - usage: make stop WORKER=gemma3-270m
stop:
	@if [ -z "$(WORKER)" ]; then \
		echo "Stopping all inference servers..."; \
		pgrep -f "inference-server" | xargs -r kill || echo "No servers running"; \
		echo "All servers stopped."; \
	else \
		if [ ! -f "envs/worker.$(WORKER).env" ]; then \
			echo "Error: envs/worker.$(WORKER).env not found"; \
			echo "Available workers: $(WORKER_NAMES)"; \
			exit 1; \
		fi; \
		port=$$(grep "HTTP_ADDR=" envs/worker.$(WORKER).env | cut -d':' -f2); \
		echo "Stopping worker $(WORKER) on port $$port..."; \
		pgrep -f "inference-server.*$(WORKER)" | xargs -r kill || echo "Worker $(WORKER) not running"; \
		echo "Worker $(WORKER) stopped."; \
	fi

# Stop all servers (legacy compatibility)
stop-all:
	@echo "Finding and stopping all inference servers..."
	@pgrep -f "inference-server" | xargs -r kill || echo "No servers running"
	@echo "All servers stopped."

# Test worker HTTP endpoints - usage: make test WORKER=gemma3-270m
test:
	@if [ -z "$(WORKER)" ]; then \
		echo "Usage: make test WORKER=<name>"; \
		echo "Available workers: $(WORKER_NAMES)"; \
		exit 1; \
	fi
	@if [ ! -f "envs/worker.$(WORKER).env" ]; then \
		echo "Error: envs/worker.$(WORKER).env not found"; \
		exit 1; \
	fi
	@port=$$(grep "HTTP_ADDR=" envs/worker.$(WORKER).env | cut -d':' -f2); \
	echo "Testing worker $(WORKER) on port $$port..."; \
	echo "Health check:"; \
	curl -s http://localhost:$$port/healthz; \
	echo ""; \
	echo "Completions test:"; \
	curl -s -X POST http://localhost:$$port/v1/completions \
		-H "Content-Type: application/json" \
		-d '{"input":"Hello, how are you?","params":{"max_tokens":50,"temperature":0.7}}' | jq .

# View worker logs - usage: make logs WORKER=gemma3-270m
logs:
	@if [ -z "$(WORKER)" ]; then \
		echo "Usage: make logs WORKER=<name>"; \
		echo "Available workers: $(WORKER_NAMES)"; \
		exit 1; \
	fi
	@port=$$(grep "HTTP_ADDR=" envs/worker.$(WORKER).env | cut -d':' -f2); \
	echo "Worker $(WORKER) request logs:"; \
	curl -s "http://localhost:$$port/logs?limit=10" | jq .


# Dynamic worker targets (for convenience)
$(WORKER_NAMES): build
	@$(MAKE) start WORKER=$@

# Dynamic test targets
$(addprefix test-,$(WORKER_NAMES)):
	@$(MAKE) test WORKER=$(patsubst test-%,%,$@)

# Dynamic log targets  
$(addprefix logs-,$(WORKER_NAMES)):
	@$(MAKE) logs WORKER=$(patsubst logs-%,%,$@)

# Dynamic stop targets
$(addprefix stop-,$(WORKER_NAMES)):
	@$(MAKE) stop WORKER=$(patsubst stop-%,%,$@)

# Show help
help:
	@echo "Available targets:"
	@echo "  build-llama              - Auto-detect and build llama.cpp with best GPU support"
	@echo "  build-llama-metal        - Build llama.cpp with Metal (macOS)"
	@echo "  build-llama-cuda         - Build llama.cpp with CUDA (NVIDIA)"
	@echo "  build-llama-rocm         - Build llama.cpp with ROCm (AMD)"
	@echo "  build-llama-vulkan       - Build llama.cpp with Vulkan"
	@echo "  build-llama-cpu          - Build llama.cpp CPU-only"
	@echo "  build                    - Build the inference server binary"
	@echo "  build-cli                - Build the NATS CLI client"
	@echo "  build-all                - Build both server and CLI"
	@echo "  clean                    - Clean build artifacts and data"
	@echo "  list-workers             - List available worker configurations"
	@echo "  start WORKER=<name>      - Start specific worker"
	@echo "  stop [WORKER=<name>]     - Stop specific worker (or all if no WORKER specified)"
	@echo "  stop-all                 - Stop all running workers"
	@echo "  test WORKER=<name>       - Test specific worker HTTP endpoints"
	@echo "  logs WORKER=<name>       - View specific worker request logs"
	@echo ""
	@echo "Convenience targets (auto-detected):"
	@for worker in $(WORKER_NAMES); do \
		echo "  $$worker                 - Start $$worker worker"; \
		echo "  stop-$$worker            - Stop $$worker worker"; \
		echo "  test-$$worker            - Test $$worker worker"; \
		echo "  logs-$$worker            - View $$worker worker logs"; \
	done
	@echo ""
	@echo "Examples:"
	@echo "  make start WORKER=gemma3-270m"
	@echo "  make gemma3-270m         # shorthand"
	@echo "  make stop WORKER=gemma3-270m"
	@echo "  make stop-gemma3-270m    # shorthand"
	@echo "  make test-gemma3-270m"
	@echo "  make logs-qwen3-4b"