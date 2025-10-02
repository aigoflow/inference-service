# Model Comparison Chat

A web-based tool for comparing multiple AI models side-by-side with custom tasks and documents.

## Features

- **Model Selection**: Select 1-5 AI models using checkboxes with real-time status indicators
- **Task Management**: Create, edit, and manage reusable task templates  
- **Document Management**: Store and manage documents for consistent testing
- **Parallel Inference**: Send requests to all selected models simultaneously
- **Performance Metrics**: View tokens in/out and response time for each model
- **Local Storage**: Tasks and documents persist in browser localStorage
- **Auto-Discovery**: Automatically detects available models via NATS heartbeat

## Getting Started

### Prerequisites

- NATS server running (default: `nats://127.0.0.1:5700`)
- At least one inference model running and publishing heartbeats

### Running the Application

1. **Start the Model Comparison Chat server:**
   ```bash
   ./bin/model-comparison-chat
   ```

2. **Open your browser and navigate to:**
   ```
   http://localhost:8878
   ```

3. **Configure and run comparisons:**
   - Select 1-5 models from the checkbox list
   - Choose or create a task template
   - Choose or create a document  
   - Set max tokens and temperature
   - Click "Compare Models" to run parallel inference

## Command Line Options

```bash
./bin/model-comparison-chat [options]
```

**Options:**
- `-addr string`: HTTP server address (default ":8878")
- `-nats string`: NATS server URL (default "nats://127.0.0.1:5700")

**Example:**
```bash
./bin/model-comparison-chat -addr :9000 -nats nats://192.168.1.100:4222
```

## Usage Guide

### 1. Model Selection
- Available models are automatically discovered via NATS heartbeats
- Green dot = online, Red dot = offline  
- Select 1-5 models using checkboxes
- Only text generation models are shown

### 2. Task Management
- **Built-in tasks**: Extract Summary, Extract Named Entities, Sentiment Analysis
- **Add custom tasks**: Use the Tasks tab to create new task templates
- **Task format**: Provide clear instructions that will be prepended to documents

### 3. Document Management  
- **Sample documents**: News article and business email provided
- **Add custom documents**: Use the Documents tab to add your own content
- **Document types**: Any text content you want to analyze

### 4. Running Comparisons
- Select models, task, and document
- Adjust max tokens (1-4096) and temperature (0-2)
- Results show side-by-side with performance metrics
- Each model runs independently with timeout handling

### 5. Results Interpretation
- **Response**: Model's generated text
- **Metrics**: Input tokens → Output tokens | Duration in ms
- **Errors**: Timeout, model unavailable, or other failures

## Architecture

The application consists of:

- **Go Backend**: HTTP server that proxies NATS requests
- **NATS Integration**: Real-time model discovery and inference requests  
- **Web Frontend**: Single-page application with tabs and local storage
- **Parallel Processing**: Concurrent requests to multiple models

## Model Requirements

Models must:
- Publish heartbeats to `monitoring.models.heartbeat.*`
- Listen for inference requests on their designated NATS subject
- Include "text" in their capabilities array
- Respond with standard inference response format

## Default Tasks

1. **Extract Summary**: "Please provide a concise summary of the key points from the following document:"

2. **Extract Named Entities**: "Extract all named entities (people, organizations, locations, dates, etc.) from the following document:"

3. **Sentiment Analysis**: "Analyze the sentiment and tone of the following document. Provide your analysis:"

## Local Storage

The application stores:
- **Tasks**: Custom task templates with IDs, names, and content
- **Documents**: Custom documents with IDs, names, and content  
- **Selection State**: Currently selected task and document

Data persists between browser sessions automatically.

## Troubleshooting

**No models appear:**
- Check NATS connection
- Verify models are running and publishing heartbeats
- Check browser console for errors

**Requests timeout:**
- Verify model endpoints are accessible
- Check model capacity and load
- Increase timeout if needed (currently 2 minutes)

**Comparison fails:**
- Ensure 1-5 models selected
- Verify task and document are selected
- Check network connectivity

## Development

To modify the application:

1. Edit `main.go` for backend changes
2. Rebuild: `go build -o bin/model-comparison-chat ./examples/model-comparison-chat/`
3. Restart the server to see changes

The HTML/CSS/JavaScript is embedded in the Go binary as a template string.