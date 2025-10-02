package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
)

type ChatRequest struct {
	Subject     string        `json:"subject"`
	Message     string        `json:"message"`
	History     []ChatMessage `json:"history"`
	MaxTokens   int           `json:"maxTokens"`
	Temperature float64       `json:"temperature"`
}

type ChatMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
	Time    string `json:"time"`
}

type InferenceRequest struct {
	ReqID   string                 `json:"req_id"`
	Input   string                 `json:"input"`
	Params  map[string]interface{} `json:"params"`
	ReplyTo string                 `json:"reply_to"`
	Raw     bool                   `json:"raw,omitempty"`
}

type InferenceResponse struct {
	ReqID        string `json:"req_id"`
	Text         string `json:"text"`
	TokensIn     int    `json:"tokens_in"`
	TokensOut    int    `json:"tokens_out"`
	FinishReason string `json:"finish_reason"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

type ChatServer struct {
	natsConn *nats.Conn
}

func NewChatServer(natsURL string) (*ChatServer, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	
	return &ChatServer{natsConn: nc}, nil
}

func (s *ChatServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Test Chat</title>
    <meta charset="utf-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #f5f5f5; }
        .header { background: #2c2c2c; color: white; padding: 20px; border-radius: 12px; margin-bottom: 20px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .main-container { display: flex; height: 80vh; background: white; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); overflow: hidden; border: 1px solid #e0e0e0; }
        .chat-area { flex: 1; display: flex; flex-direction: column; }
        .chat-history { flex: 1; padding: 20px; overflow-y: auto; background: #fafafa; }
        .chat-input-area { border-top: 1px solid #e0e0e0; padding: 20px; background: white; }
        .input-row { display: flex; gap: 12px; align-items: flex-end; }
        .sidebar { width: 300px; padding: 20px; background: #f8f8f8; border-left: 1px solid #e0e0e0; }
        .control-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: 600; color: #333; font-size: 0.9em; }
        select, input[type="number"], input[type="text"] { width: 100%; padding: 8px 12px; border: 1px solid #d0d0d0; border-radius: 6px; box-sizing: border-box; font-size: 14px; background: white; }
        .model-selector select { height: auto; }
        .model-option { display: flex; align-items: center; }
        .status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 8px; }
        .status-online { background-color: #4caf50; }
        .status-offline { background-color: #f44336; }
        .status-unknown { background-color: #999; }
        select:focus, input:focus, textarea:focus { outline: none; border-color: #ff8c00; box-shadow: 0 0 0 2px rgba(255, 140, 0, 0.1); }
        #messageInput { flex: 1; padding: 12px; border: 1px solid #d0d0d0; border-radius: 8px; resize: none; min-height: 20px; max-height: 120px; font-family: inherit; font-size: 14px; background: white; }
        button { background: #ff8c00; color: white; padding: 12px 24px; border: none; border-radius: 8px; cursor: pointer; font-weight: 600; transition: all 0.2s; }
        button:hover { background: #ff7700; transform: translateY(-1px); box-shadow: 0 4px 8px rgba(255, 140, 0, 0.3); }
        button:disabled { background: #999; cursor: not-allowed; transform: none; box-shadow: none; }
        .message { margin-bottom: 16px; max-width: 80%; }
        .user-message { background: #333; color: white; padding: 12px 16px; border-radius: 18px 18px 4px 18px; margin-left: auto; }
        .assistant-message { background: #e8e8e8; color: #2c2c2c; padding: 12px 16px; border-radius: 18px 18px 18px 4px; margin-right: auto; border: 1px solid #d0d0d0; }
        .error-message { background: #ffe6e6; color: #cc0000; padding: 12px 16px; border-radius: 8px; border: 1px solid #ffb3b3; }
        .message-time { font-size: 0.75em; color: rgba(255,255,255,0.8); margin-bottom: 4px; }
        .assistant-message .message-time { color: #666; }
        .error-message .message-time { color: #cc0000; }
        .message-content { line-height: 1.4; }
        .message-content h1, .message-content h2, .message-content h3 { margin: 0.5em 0; }
        .message-content h1 { font-size: 1.2em; font-weight: bold; }
        .message-content h2 { font-size: 1.1em; font-weight: bold; }
        .message-content h3 { font-size: 1.05em; font-weight: bold; }
        .message-content p { margin: 0.5em 0; }
        .message-content ul, .message-content ol { margin: 0.5em 0; padding-left: 1.5em; }
        .message-content li { margin: 0.2em 0; }
        .message-content code { background: rgba(0,0,0,0.1); padding: 2px 4px; border-radius: 3px; font-family: 'Monaco', 'Consolas', monospace; font-size: 0.9em; }
        .message-content pre { background: rgba(0,0,0,0.05); padding: 12px; border-radius: 6px; overflow-x: auto; border-left: 3px solid #ff8c00; }
        .message-content pre code { background: none; padding: 0; }
        .message-content blockquote { border-left: 3px solid #ff8c00; padding-left: 12px; margin: 0.5em 0; font-style: italic; }
        .message-content strong { font-weight: bold; }
        .message-content em { font-style: italic; }
        .message-content a { color: #ff8c00; text-decoration: none; }
        .message-content a:hover { text-decoration: underline; }
        .status { padding: 12px 16px; background: #fff4e6; border: 1px solid #ffcc80; border-radius: 8px; margin: 10px 0; font-size: 0.9em; color: #e65100; }
        .metrics { font-size: 0.75em; color: rgba(255,255,255,0.7); margin-top: 6px; }
        .assistant-message .metrics { color: #666; }
        #clearHistory { background: #666; margin-top: 15px; }
        #clearHistory:hover { background: #555; transform: translateY(-1px); }
    </style>
</head>
<body>
    <div class="header">
        <h1>🤖 Test Chat</h1>
        <p>Test text generation and embedding models via NATS messaging with chat history</p>
    </div>

    <div class="main-container">
        <div class="chat-area">
            <div class="chat-history" id="chatHistory">
                <div class="message assistant-message">
                    <div class="message-time">System</div>
                    <div class="message-content">Welcome! Select a model and start chatting. Your conversation history will be included in requests.</div>
                </div>
            </div>
            
            <div class="chat-input-area">
                <div class="input-row">
                    <textarea id="messageInput" placeholder="Type your message here..." rows="1"></textarea>
                    <button onclick="sendMessage()" id="sendButton">Send</button>
                </div>
                <div id="status" class="status" style="display: none;"></div>
            </div>
        </div>

        <div class="sidebar">
            <div class="control-group">
                <label for="subject">Model:</label>
                <select id="subject" data-live-search="true">
                    <option value="">🔍 Loading models...</option>
                </select>
            </div>

            <div class="control-group">
                <label for="maxTokens">Max Tokens:</label>
                <input type="number" id="maxTokens" value="500" min="1" max="4096">
            </div>

            <div class="control-group">
                <label for="temperature">Temperature:</label>
                <input type="number" id="temperature" value="0.7" min="0" max="2" step="0.1">
            </div>

            <button onclick="clearHistory()" id="clearHistory">Clear History</button>
        </div>
    </div>

    <script>
        let chatHistory = [];
        let discoveredModels = new Map();
        let natsConnection = null;

        // Load saved model from localStorage
        function loadSavedModel() {
            const saved = localStorage.getItem('selectedModel');
            if (saved) {
                const select = document.getElementById('subject');
                select.value = saved;
            }
        }

        // Save selected model to localStorage
        function saveSelectedModel() {
            const select = document.getElementById('subject');
            localStorage.setItem('selectedModel', select.value);
        }

        // Connect to NATS for heartbeat monitoring
        async function connectToNATS() {
            try {
                // Use EventSource for real-time model updates via SSE
                const eventSource = new EventSource('/models/stream');
                
                eventSource.onmessage = function(event) {
                    const modelData = JSON.parse(event.data);
                    updateDiscoveredModel(modelData);
                };
                
                eventSource.onerror = function(error) {
                    console.log('SSE connection error:', error);
                    // Fallback to polling
                    setTimeout(pollModels, 5000);
                };
            } catch (error) {
                console.log('Failed to connect SSE, using polling fallback');
                pollModels();
            }
        }

        // Fallback polling method
        async function pollModels() {
            try {
                const response = await fetch('/models');
                const models = await response.json();
                models.forEach(model => updateDiscoveredModel(model));
            } catch (error) {
                console.log('Failed to poll models:', error);
            }
            setTimeout(pollModels, 10000); // Poll every 10 seconds
        }

        function updateDiscoveredModel(modelData) {
            const key = modelData.model_name;
            const lastSeen = new Date();
            
            // Store in discovered models
            discoveredModels.set(key, {
                ...modelData,
                lastSeen: lastSeen,
                isOnline: (Date.now() - new Date(modelData.last_activity).getTime()) < 60000 // Online if heartbeat within 1 minute
            });
            
            // Store in localStorage
            const stored = JSON.parse(localStorage.getItem('allModels') || '{}');
            stored[key] = { ...modelData, lastSeen: lastSeen.toISOString() };
            localStorage.setItem('allModels', JSON.stringify(stored));
            
            updateModelDropdown();
        }

        function updateModelDropdown(searchTerm = '') {
            const select = document.getElementById('subject');
            const search = searchTerm.toLowerCase();
            const currentSelection = select.value;
            
            // Get all models (discovered + stored)
            const allModels = new Map(discoveredModels);
            const stored = JSON.parse(localStorage.getItem('allModels') || '{}');
            
            Object.entries(stored).forEach(([key, model]) => {
                if (!allModels.has(key)) {
                    allModels.set(key, { ...model, isOnline: false, lastSeen: new Date(model.lastSeen) });
                }
            });
            
            // Filter and sort models
            const filteredModels = Array.from(allModels.values())
                .filter(model => 
                    search === '' || 
                    model.model_name.toLowerCase().includes(search) ||
                    (model.model_info && model.model_info.architecture && model.model_info.architecture.toLowerCase().includes(search))
                )
                .sort((a, b) => {
                    // Sort by: online first, then alphabetically
                    if (a.isOnline !== b.isOnline) return b.isOnline - a.isOnline;
                    return a.model_name.localeCompare(b.model_name);
                });
            
            // Clear and rebuild dropdown
            select.innerHTML = '';
            
            if (filteredModels.length === 0) {
                select.innerHTML = '<option value="">No models found</option>';
                return;
            }
            
            filteredModels.forEach(model => {
                const option = document.createElement('option');
                const statusDot = model.isOnline ? '🟢' : '🔴';
                const capabilities = model.capabilities ? model.capabilities.join(', ') : 'unknown';
                const paramCount = model.model_info && model.model_info.parameter_count ? model.model_info.parameter_count : 'unknown';
                
                option.value = model.nats_topic;
                option.textContent = statusDot + ' ' + model.model_name + ' (' + paramCount + ')';
                option.title = 'Capabilities: ' + capabilities + '\\nArchitecture: ' + (model.model_info?.architecture || 'unknown') + '\\nEndpoint: ' + (model.endpoint || 'unknown');
                
                select.appendChild(option);
            });
            
            // Restore selection if still valid
            if (currentSelection && Array.from(select.options).some(opt => opt.value === currentSelection)) {
                select.value = currentSelection;
            } else if (filteredModels.length > 0) {
                // Select first online model or first model
                const firstOnline = filteredModels.find(m => m.isOnline);
                if (firstOnline) {
                    select.value = firstOnline.nats_topic;
                }
            }
        }

        function addMessage(role, content, metrics = null) {
            const time = new Date().toLocaleTimeString();
            const message = { role, content, time, metrics };
            chatHistory.push(message);
            
            const historyDiv = document.getElementById('chatHistory');
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + (role === 'user' ? 'user-message' : 'assistant-message');
            
            let metricsHtml = '';
            if (metrics) {
                metricsHtml = '<div class="metrics">' + 
                    'Tokens: ' + metrics.tokens_in + '→' + metrics.tokens_out + 
                    ' | Duration: ' + metrics.duration_ms + 'ms</div>';
            }
            
            // Use markdown rendering for assistant messages, plain text for user messages
            const contentHtml = role === 'assistant' ? renderMarkdown(content) : escapeHtml(content).replace(/\n/g, '<br>');
            
            messageDiv.innerHTML = 
                '<div class="message-time">' + time + ' (' + role + ')</div>' +
                '<div class="message-content">' + contentHtml + '</div>' +
                metricsHtml;
            
            historyDiv.appendChild(messageDiv);
            historyDiv.scrollTop = historyDiv.scrollHeight;
        }

        function addErrorMessage(error) {
            const time = new Date().toLocaleTimeString();
            const historyDiv = document.getElementById('chatHistory');
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message error-message';
            messageDiv.innerHTML = 
                '<div class="message-time">' + time + ' (Error)</div>' +
                '<div class="message-content">' + escapeHtml(error) + '</div>';
            historyDiv.appendChild(messageDiv);
            historyDiv.scrollTop = historyDiv.scrollHeight;
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function renderMarkdown(text) {
            let html = escapeHtml(text);
            
            // Convert line breaks
            html = html.replace(/\n/g, '<br>');
            
            // Code blocks - triple backticks using string constructor  
            const tripleBacktick = String.fromCharCode(96) + String.fromCharCode(96) + String.fromCharCode(96);
            const codeBlockRegex = new RegExp(tripleBacktick + '([\\s\\S]*?)' + tripleBacktick, 'g');
            html = html.replace(codeBlockRegex, '<pre><code>$1</code></pre>');
            
            // Inline code - single backticks using string constructor
            const backtickRegex = new RegExp(String.fromCharCode(96) + '([^' + String.fromCharCode(96) + ']+)' + String.fromCharCode(96), 'g');
            html = html.replace(backtickRegex, '<code>$1</code>');
            
            // Bold - double asterisks
            const boldRegex = new RegExp('\\*\\*([^*]+)\\*\\*', 'g');
            html = html.replace(boldRegex, '<strong>$1</strong>');
            
            // Headers
            html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
            html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
            html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
            
            // Lists
            html = html.replace(/^- (.+)$/gm, '<li>$1</li>');
            
            return html;
        }

        function buildPromptWithHistory(newMessage) {
            // For first few messages, just use direct input to avoid template conflicts
            if (chatHistory.length <= 2) {
                return newMessage;
            }
            
            // Build very simple context - just the last assistant response and new message
            const lastAssistant = chatHistory
                .slice()
                .reverse()
                .find(msg => msg.role === 'assistant');
            
            if (lastAssistant) {
                return 'Context: Previously I said "' + lastAssistant.content + '"\n\nNow you say: ' + newMessage;
            }
            
            return newMessage;
        }

        async function sendMessage() {
            const subject = document.getElementById('subject').value;
            const message = document.getElementById('messageInput').value.trim();
            const maxTokens = parseInt(document.getElementById('maxTokens').value);
            const temperature = parseFloat(document.getElementById('temperature').value);
            
            if (!message) {
                alert('Please enter a message');
                return;
            }

            const sendButton = document.getElementById('sendButton');
            const status = document.getElementById('status');
            
            sendButton.disabled = true;
            status.style.display = 'block';
            status.textContent = 'Sending request via NATS...';

            // Add user message to history
            addMessage('user', message);
            
            // Build prompt with history for text generation, or use direct input for embeddings
            let requestInput;
            if (subject.includes('embedding.request')) {
                requestInput = message; // Direct input for embeddings
            } else {
                requestInput = buildPromptWithHistory(message);
                console.log('Built prompt:', requestInput); // Debug logging
            }

            try {
                const response = await fetch('/chat', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        subject: subject,
                        message: requestInput,
                        history: chatHistory,
                        maxTokens: maxTokens,
                        temperature: temperature
                    })
                });

                const result = await response.json();
                
                if (result.error) {
                    addErrorMessage('Error: ' + result.error);
                } else {
                    if (subject.includes('embedding.request')) {
                        // Handle embedding response
                        const embData = result.data && result.data[0];
                        if (embData) {
                            const embText = 'Generated ' + embData.embedding.length + '-dimensional embedding vector\\n' +
                                          'First 10 values: [' + embData.embedding.slice(0, 10).map(v => v.toFixed(4)).join(', ') + '...]';
                            addMessage('assistant', embText, { 
                                tokens_in: result.usage.prompt_tokens, 
                                tokens_out: 0,
                                duration_ms: result.duration_ms || 0
                            });
                        }
                    } else {
                        // Handle text generation response
                        addMessage('assistant', result.text, {
                            tokens_in: result.tokens_in,
                            tokens_out: result.tokens_out,
                            duration_ms: result.duration_ms
                        });
                    }
                }
            } catch (error) {
                addErrorMessage('Network error: ' + error.message);
            } finally {
                sendButton.disabled = false;
                status.style.display = 'none';
                document.getElementById('messageInput').value = '';
            }
        }

        function clearHistory() {
            if (confirm('Clear chat history?')) {
                chatHistory = [];
                const historyDiv = document.getElementById('chatHistory');
                historyDiv.innerHTML = 
                    '<div class="message assistant-message">' +
                    '<div class="message-time">System</div>' +
                    '<div class="message-content">Chat history cleared. Start a new conversation!</div>' +
                    '</div>';
            }
        }

        // Initialize on page load
        document.addEventListener('DOMContentLoaded', function() {
            // Load stored models first, then start real-time updates
            loadStoredModels();
            loadSavedModel();
            connectToNATS();
            
            // Model selection change handler
            document.getElementById('subject').addEventListener('change', saveSelectedModel);
        });

        // Load models from localStorage immediately
        function loadStoredModels() {
            const stored = JSON.parse(localStorage.getItem('allModels') || '{}');
            Object.entries(stored).forEach(([key, model]) => {
                // Mark stored models as offline until heartbeat confirms otherwise
                discoveredModels.set(key, { ...model, isOnline: false, lastSeen: new Date(model.lastSeen) });
            });
            updateModelDropdown();
        }

        // Auto-resize textarea and Enter key to send
        const messageInput = document.getElementById('messageInput');
        messageInput.addEventListener('input', function() {
            this.style.height = 'auto';
            this.style.height = Math.min(this.scrollHeight, 120) + 'px';
        });
        
        messageInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });
    </script>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, nil)
}

func (s *ChatServer) handleModels(w http.ResponseWriter, r *http.Request) {
	// Simple polling endpoint - just return empty for now
	// In a real implementation, this would connect to NATS and get current heartbeats
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[]"))
}

func (s *ChatServer) handleModelsStream(w http.ResponseWriter, r *http.Request) {
	// Set up Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Subscribe to heartbeat topic
	sub, err := s.natsConn.Subscribe("monitoring.models.heartbeat.*", func(msg *nats.Msg) {
		// Forward heartbeat data as SSE
		fmt.Fprintf(w, "data: %s\n\n", string(msg.Data))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	if err != nil {
		http.Error(w, "Failed to subscribe to heartbeats", http.StatusInternalServerError)
		return
	}
	defer sub.Unsubscribe()

	// Keep connection alive
	<-r.Context().Done()
}

func (s *ChatServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Determine if this is an embedding or inference request
	isEmbedding := len(req.Subject) > 0 && req.Subject[:9] == "embedding"
	
	if isEmbedding {
		s.handleEmbeddingRequest(w, req)
	} else {
		s.handleInferenceRequest(w, req)
	}
}

func (s *ChatServer) handleInferenceRequest(w http.ResponseWriter, req ChatRequest) {
	// Create NATS inference request
	replySubject := fmt.Sprintf("webchat.reply.%d", time.Now().UnixNano())
	
	natsReq := InferenceRequest{
		ReqID:   fmt.Sprintf("webchat-%d", time.Now().UnixNano()),
		Input:   req.Message,
		ReplyTo: replySubject,
		Raw:     false, // Let template system handle formatting
		Params: map[string]interface{}{
			"max_tokens":  req.MaxTokens,
			"temperature": req.Temperature,
		},
	}

	// Subscribe to reply
	replyChan := make(chan *nats.Msg, 1)
	sub, err := s.natsConn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to subscribe: %v", err), http.StatusInternalServerError)
		return
	}
	defer sub.Unsubscribe()

	// Send request
	reqBytes, _ := json.Marshal(natsReq)
	if err := s.natsConn.Publish(req.Subject, reqBytes); err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait for response
	select {
	case msg := <-replyChan:
		var response InferenceResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse response: %v", err), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
	case <-time.After(2 * time.Minute):
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
	}
}

func (s *ChatServer) handleEmbeddingRequest(w http.ResponseWriter, req ChatRequest) {
	// Create embedding request structure
	type EmbeddingRequest struct {
		ReqID   string      `json:"req_id"`
		Input   interface{} `json:"input"`
		Model   string      `json:"model"`
		ReplyTo string      `json:"reply_to"`
	}

	// Create NATS embedding request  
	replySubject := fmt.Sprintf("webchat.embed.reply.%d", time.Now().UnixNano())
	
	natsReq := EmbeddingRequest{
		ReqID:   fmt.Sprintf("webchat-embed-%d", time.Now().UnixNano()),
		Input:   req.Message,
		Model:   "nomic-embed-v1.5", // Default model
		ReplyTo: replySubject,
	}

	// Subscribe to reply
	replyChan := make(chan *nats.Msg, 1)
	sub, err := s.natsConn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to subscribe: %v", err), http.StatusInternalServerError)
		return
	}
	defer sub.Unsubscribe()

	// Send request
	reqBytes, _ := json.Marshal(natsReq)
	if err := s.natsConn.Publish(req.Subject, reqBytes); err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait for response
	select {
	case msg := <-replyChan:
		// Forward the embedding response directly
		w.Header().Set("Content-Type", "application/json")
		w.Write(msg.Data)
		
	case <-time.After(2 * time.Minute):
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
	}
}

func (s *ChatServer) Close() {
	if s.natsConn != nil {
		s.natsConn.Close()
	}
}

func main() {
	var (
		natsURL = flag.String("nats", "nats://127.0.0.1:5700", "NATS server URL")
		httpAddr = flag.String("addr", ":8877", "HTTP server address")
	)
	flag.Parse()

	// Create chat server
	server, err := NewChatServer(*natsURL)
	if err != nil {
		log.Fatalf("Failed to create chat server: %v", err)
	}
	defer server.Close()

	// Setup routes
	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/chat", server.handleChat)
	http.HandleFunc("/models", server.handleModels)
	http.HandleFunc("/models/stream", server.handleModelsStream)

	fmt.Printf("🚀 Starting web chat UI on http://localhost%s\n", *httpAddr)
	fmt.Printf("📡 Connected to NATS: %s\n", *natsURL)
	fmt.Println("Open your browser and start testing!")

	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}