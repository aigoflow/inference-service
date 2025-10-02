package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type ComparisonRequest struct {
	Models      []string `json:"models"`
	Task        string   `json:"task"`
	Document    string   `json:"document"`
	MaxTokens   int      `json:"maxTokens"`
	Temperature float64  `json:"temperature"`
}

type ModelResult struct {
	ModelName    string `json:"modelName"`
	Response     string `json:"response"`
	TokensIn     int    `json:"tokensIn"`
	TokensOut    int    `json:"tokensOut"`
	DurationMs   int64  `json:"durationMs"`
	Error        string `json:"error,omitempty"`
}

type ComparisonResponse struct {
	Results []ModelResult `json:"results"`
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

type ComparisonServer struct {
	natsConn *nats.Conn
}

func NewComparisonServer(natsURL string) (*ComparisonServer, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	
	return &ComparisonServer{natsConn: nc}, nil
}

func (s *ComparisonServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Model Comparison Chat</title>
    <meta charset="utf-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 1400px; margin: 0 auto; padding: 20px; background: #f5f5f5; }
        .header { background: #2c2c2c; color: white; padding: 20px; border-radius: 12px; margin-bottom: 20px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
        .main-container { display: flex; height: 85vh; background: white; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); overflow: hidden; border: 1px solid #e0e0e0; }
        .content-area { flex: 1; display: flex; flex-direction: column; }
        .tabs { display: flex; background: #f8f8f8; border-bottom: 1px solid #e0e0e0; }
        .tab { padding: 12px 24px; cursor: pointer; border: none; background: none; font-size: 14px; font-weight: 500; color: #666; transition: all 0.2s; }
        .tab:hover { background: #e8e8e8; color: #333; }
        .tab.active { background: white; color: #ff8c00; border-bottom: 2px solid #ff8c00; }
        .tab-content { flex: 1; padding: 20px; overflow-y: auto; display: none; }
        .tab-content.active { display: block; }
        .sidebar { width: 300px; padding: 20px; background: #f8f8f8; border-left: 1px solid #e0e0e0; overflow-y: auto; }
        .control-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: 600; color: #333; font-size: 0.9em; }
        select, input[type="number"], input[type="text"], textarea { width: 100%; padding: 8px 12px; border: 1px solid #d0d0d0; border-radius: 6px; box-sizing: border-box; font-size: 14px; background: white; font-family: inherit; }
        textarea { min-height: 100px; resize: vertical; }
        .model-checkboxes { max-height: 200px; overflow-y: auto; border: 1px solid #d0d0d0; border-radius: 6px; padding: 8px; background: white; }
        .sidebar select { position: relative; z-index: 1000; }
        .model-option { display: flex; align-items: center; margin-bottom: 8px; }
        .model-option input[type="checkbox"] { margin-right: 8px; }
        .status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 8px; }
        .status-online { background-color: #4caf50; }
        .status-offline { background-color: #f44336; }
        button { background: #ff8c00; color: white; padding: 12px 24px; border: none; border-radius: 8px; cursor: pointer; font-weight: 600; transition: all 0.2s; width: 100%; font-size: 14px; }
        button:hover { background: #ff7700; transform: translateY(-1px); box-shadow: 0 4px 8px rgba(255, 140, 0, 0.3); }
        button:disabled { background: #999; cursor: not-allowed; transform: none; box-shadow: none; }
        .comparison-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 20px; }
        .model-result { background: #f9f9f9; border: 1px solid #e0e0e0; border-radius: 8px; padding: 15px; }
        .model-result.error { background: #ffe6e6; border-color: #ffb3b3; }
        .model-header { display: flex; justify-content: between; align-items: center; margin-bottom: 10px; }
        .model-name { font-weight: bold; color: #333; }
        .model-metrics { font-size: 0.8em; color: #666; }
        .model-response { background: white; border: 1px solid #ddd; border-radius: 6px; padding: 12px; font-family: inherit; line-height: 1.4; max-height: 300px; overflow-y: auto; }
        .model-response h1, .model-response h2, .model-response h3 { margin: 0.5em 0; }
        .model-response h1 { font-size: 1.2em; font-weight: bold; }
        .model-response h2 { font-size: 1.1em; font-weight: bold; }
        .model-response h3 { font-size: 1.05em; font-weight: bold; }
        .model-response p { margin: 0.5em 0; }
        .model-response ul, .model-response ol { margin: 0.5em 0; padding-left: 1.5em; }
        .model-response li { margin: 0.2em 0; }
        .model-response code { background: rgba(0,0,0,0.1); padding: 2px 4px; border-radius: 3px; font-family: 'Monaco', 'Consolas', monospace; font-size: 0.9em; }
        .model-response pre { background: rgba(0,0,0,0.05); padding: 12px; border-radius: 6px; overflow-x: auto; border-left: 3px solid #ff8c00; }
        .model-response pre code { background: none; padding: 0; }
        .model-response strong { font-weight: bold; }
        .model-response em { font-style: italic; }
        .loading-spinner { display: inline-block; width: 20px; height: 20px; border: 2px solid #f3f3f3; border-top: 2px solid #ff8c00; border-radius: 50%; animation: spin 1s linear infinite; }
        @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        .task-item, .document-item { background: white; border: 1px solid #ddd; border-radius: 6px; padding: 12px; margin-bottom: 10px; cursor: pointer; transition: all 0.2s; }
        .task-item:hover, .document-item:hover { background: #f5f5f5; border-color: #ff8c00; }
        .task-item.selected, .document-item.selected { background: #fff4e6; border-color: #ff8c00; }
        .item-actions { margin-top: 8px; display: flex; gap: 8px; }
        .item-actions button { padding: 4px 8px; font-size: 12px; width: auto; }
        .item-actions .delete { background: #f44336; }
        .item-actions .delete:hover { background: #d32f2f; }
        .add-item-form { background: white; border: 1px solid #ddd; border-radius: 6px; padding: 15px; margin-bottom: 15px; }
        .status { padding: 12px 16px; background: #fff4e6; border: 1px solid #ffcc80; border-radius: 8px; margin: 10px 0; font-size: 0.9em; color: #e65100; display: none; }
        .edit-sidebar { position: fixed; top: 0; right: -600px; width: 600px; height: 100vh; background: white; box-shadow: -4px 0 8px rgba(0,0,0,0.2); z-index: 1000; transition: right 0.3s ease; display: flex; flex-direction: column; }
        .edit-sidebar.open { right: 0; }
        .edit-header { display: flex; justify-content: space-between; align-items: center; padding: 20px; border-bottom: 1px solid #e0e0e0; background: #f8f8f8; }
        .edit-header h3 { margin: 0; color: #333; }
        .close-btn { background: none; border: none; font-size: 24px; cursor: pointer; color: #666; padding: 0; width: 30px; height: 30px; display: flex; align-items: center; justify-content: center; }
        .close-btn:hover { background: #f0f0f0; border-radius: 50%; }
        .edit-content { flex: 1; padding: 20px; overflow-y: auto; }
        .edit-actions { padding: 20px; border-top: 1px solid #e0e0e0; display: flex; gap: 10px; }
        .edit-actions button { flex: 1; }
        .edit-actions .cancel { background: #666; }
        .edit-actions .cancel:hover { background: #555; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔍 Model Comparison Chat</h1>
        <p>Compare multiple AI models side-by-side with custom tasks and documents</p>
    </div>

    <div class="main-container">
        <div class="content-area">
            <div class="tabs">
                <button class="tab active" onclick="switchTab('comparison')">Comparison</button>
                <button class="tab" onclick="switchTab('tasks')">Tasks</button>
                <button class="tab" onclick="switchTab('documents')">Documents</button>
            </div>
            
            <div class="tab-content active" id="comparison-tab">
                <div id="status" class="status"></div>
                <div id="comparisonResults" class="comparison-grid"></div>
            </div>
            
            <div class="tab-content" id="tasks-tab">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
                    <h3>Manage Tasks</h3>
                    <button onclick="openTaskEditor()">+ Add New Task</button>
                </div>
                <div id="tasksList"></div>
            </div>
            
            <div class="tab-content" id="documents-tab">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
                    <h3>Manage Documents</h3>
                    <button onclick="openDocumentEditor()">+ Add New Document</button>
                </div>
                <div id="documentsList"></div>
            </div>
        </div>

        <div class="sidebar">
            <div class="control-group">
                <label>Select Models (1-5):</label>
                <div class="model-checkboxes" id="modelCheckboxes">
                    <div>🔍 Loading models...</div>
                </div>
            </div>

            <div class="control-group">
                <label for="taskSelector">Task:</label>
                <select id="taskSelector">
                    <option value="">Select a task...</option>
                </select>
            </div>

            <div class="control-group">
                <label for="documentSelector">Document:</label>
                <select id="documentSelector">
                    <option value="">Select a document...</option>
                </select>
            </div>

            <div class="control-group">
                <label for="maxTokens">Max Tokens:</label>
                <input type="number" id="maxTokens" value="1500" min="1" max="4096">
            </div>

            <div class="control-group">
                <label for="temperature">Temperature:</label>
                <input type="number" id="temperature" value="0.7" min="0" max="2" step="0.1">
            </div>

            <button onclick="runComparison()" id="compareButton">Compare Models</button>
        </div>
        
        <!-- Edit Sidebar -->
        <div class="edit-sidebar" id="editSidebar">
            <div class="edit-header">
                <h3 id="editTitle">Edit Item</h3>
                <button onclick="closeEditor()" class="close-btn">×</button>
            </div>
            <div class="edit-content">
                <div class="control-group">
                    <label for="editName">Name:</label>
                    <input type="text" id="editName" placeholder="Enter name">
                </div>
                <div class="control-group">
                    <label for="editContent">Content:</label>
                    <textarea id="editContent" placeholder="Enter content" rows="20"></textarea>
                </div>
                <div class="edit-actions">
                    <button onclick="saveEdit()" id="saveButton">Save</button>
                    <button onclick="closeEditor()" class="cancel">Cancel</button>
                </div>
            </div>
        </div>
    </div>

    <script>
        let discoveredModels = new Map();
        let tasks = JSON.parse(localStorage.getItem('comparisonTasks') || '[]');
        let documents = JSON.parse(localStorage.getItem('comparisonDocuments') || '[]');
        let selectedTask = '';
        let selectedDocument = '';
        let editingItem = null;
        let editingType = null;

        // Initialize default tasks and documents
        if (tasks.length === 0) {
            tasks = [
                {
                    id: 'extract-summary',
                    name: 'Extract Summary',
                    content: 'Please provide a concise summary of the key points from the following document:\n\n'
                },
                {
                    id: 'extract-entities',
                    name: 'Extract Named Entities',
                    content: 'Extract all named entities (people, organizations, locations, dates, etc.) from the following document:\n\n'
                },
                {
                    id: 'sentiment-analysis',
                    name: 'Sentiment Analysis',
                    content: 'Analyze the sentiment and tone of the following document. Provide your analysis:\n\n'
                }
            ];
            saveTasks();
        }

        if (documents.length === 0) {
            documents = [
                {
                    id: 'sample-news',
                    name: 'Sample News Article',
                    content: 'BREAKING: Tech Giant Announces Revolutionary AI Breakthrough\n\nSan Francisco, CA - In a groundbreaking announcement today, leading technology company InnovateTech revealed their latest artificial intelligence system, dubbed "NextGen AI," which promises to revolutionize how businesses process and analyze data.\n\nThe new system, developed over three years by a team of 200+ engineers and researchers, uses advanced machine learning algorithms to process unstructured data at unprecedented speeds. According to CEO Sarah Johnson, "NextGen AI can analyze complex documents, images, and audio files in real-time, providing insights that would take human analysts weeks to discover."\n\nThe announcement has sent shockwaves through the tech industry, with competitors scrambling to understand the implications. Stock prices for InnovateTech surged 15% in after-hours trading following the news.\n\nThe company plans to begin beta testing with select enterprise customers next quarter, with a full commercial launch expected by year-end.'
                },
                {
                    id: 'sample-email',
                    name: 'Sample Business Email',
                    content: 'Subject: Quarterly Sales Review Meeting - Action Required\n\nDear Team,\n\nI hope this email finds you well. As we approach the end of Q3, it\'s time for our quarterly sales review meeting to assess our progress and plan for Q4.\n\nMeeting Details:\n- Date: October 15th, 2024\n- Time: 2:00 PM - 4:00 PM PST\n- Location: Conference Room A / Zoom link will be provided\n- Attendees: All sales team members, regional managers, and C-suite executives\n\nAgenda:\n1. Q3 Performance Analysis\n2. Individual territory reviews\n3. Market trends and competitive analysis\n4. Q4 forecasting and goal setting\n5. New product launch strategies\n\nPlease prepare:\n- Your Q3 sales reports\n- Territory analysis with key wins and challenges\n- Q4 pipeline overview\n- Any questions or concerns for discussion\n\nIf you cannot attend, please send your delegate and ensure all materials are shared in advance.\n\nLooking forward to a productive session.\n\nBest regards,\nMichael Chen\nVP of Sales'
                }
            ];
            saveDocuments();
        }

        function switchTab(tabName) {
            // Hide all tabs
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
            
            // Show selected tab
            event.target.classList.add('active');
            document.getElementById(tabName + '-tab').classList.add('active');
            
            // Refresh content if needed
            if (tabName === 'tasks') updateTasksList();
            if (tabName === 'documents') updateDocumentsList();
        }

        function saveTasks() {
            localStorage.setItem('comparisonTasks', JSON.stringify(tasks));
            updateTaskSelector();
        }

        function saveDocuments() {
            localStorage.setItem('comparisonDocuments', JSON.stringify(documents));
            updateDocumentSelector();
        }

        function openTaskEditor(taskId = null) {
            editingType = 'task';
            const sidebar = document.getElementById('editSidebar');
            const title = document.getElementById('editTitle');
            const nameInput = document.getElementById('editName');
            const contentInput = document.getElementById('editContent');
            
            if (taskId) {
                editingItem = tasks.find(t => t.id === taskId);
                title.textContent = 'Edit Task';
                nameInput.value = editingItem.name;
                contentInput.value = editingItem.content;
            } else {
                editingItem = null;
                title.textContent = 'Add New Task';
                nameInput.value = '';
                contentInput.value = '';
            }
            
            sidebar.classList.add('open');
        }

        function openDocumentEditor(docId = null) {
            editingType = 'document';
            const sidebar = document.getElementById('editSidebar');
            const title = document.getElementById('editTitle');
            const nameInput = document.getElementById('editName');
            const contentInput = document.getElementById('editContent');
            
            if (docId) {
                editingItem = documents.find(d => d.id === docId);
                title.textContent = 'Edit Document';
                nameInput.value = editingItem.name;
                contentInput.value = editingItem.content;
            } else {
                editingItem = null;
                title.textContent = 'Add New Document';
                nameInput.value = '';
                contentInput.value = '';
            }
            
            sidebar.classList.add('open');
        }

        function closeEditor() {
            document.getElementById('editSidebar').classList.remove('open');
            editingItem = null;
            editingType = null;
        }

        function saveEdit() {
            const name = document.getElementById('editName').value.trim();
            const content = document.getElementById('editContent').value.trim();
            
            if (!name || !content) {
                alert('Please enter both name and content');
                return;
            }
            
            if (editingType === 'task') {
                if (editingItem) {
                    // Edit existing task
                    editingItem.name = name;
                    editingItem.content = content;
                } else {
                    // Add new task
                    const task = {
                        id: 'task-' + Date.now(),
                        name: name,
                        content: content
                    };
                    tasks.push(task);
                }
                saveTasks();
                updateTasksList();
            } else if (editingType === 'document') {
                if (editingItem) {
                    // Edit existing document
                    editingItem.name = name;
                    editingItem.content = content;
                } else {
                    // Add new document
                    const doc = {
                        id: 'doc-' + Date.now(),
                        name: name,
                        content: content
                    };
                    documents.push(doc);
                }
                saveDocuments();
                updateDocumentsList();
            }
            
            closeEditor();
        }

        function deleteTask(taskId) {
            if (confirm('Delete this task?')) {
                tasks = tasks.filter(t => t.id !== taskId);
                saveTasks();
                updateTasksList();
            }
        }

        function editTask(taskId) {
            openTaskEditor(taskId);
        }

        function editDocument(docId) {
            openDocumentEditor(docId);
        }

        function deleteDocument(docId) {
            if (confirm('Delete this document?')) {
                documents = documents.filter(d => d.id !== docId);
                saveDocuments();
                updateDocumentsList();
            }
        }

        function updateTasksList() {
            const container = document.getElementById('tasksList');
            container.innerHTML = '';
            tasks.forEach(task => {
                const div = document.createElement('div');
                div.className = 'task-item' + (task.id === selectedTask ? ' selected' : '');
                div.onclick = () => selectTask(task.id);
                div.innerHTML = '<strong>' + task.name + '</strong>' +
                    '<div style="font-size: 0.9em; color: #666; margin-top: 4px; max-height: 50px; overflow: hidden;">' + 
                    task.content.substring(0, 150) + (task.content.length > 150 ? '...' : '') + '</div>' +
                    '<div class="item-actions">' +
                    '<button onclick="event.stopPropagation(); editTask(\'' + task.id + '\')">Edit</button>' +
                    '<button class="delete" onclick="event.stopPropagation(); deleteTask(\'' + task.id + '\')">Delete</button></div>';
                container.appendChild(div);
            });
        }

        function updateDocumentsList() {
            const container = document.getElementById('documentsList');
            container.innerHTML = '';
            documents.forEach(doc => {
                const div = document.createElement('div');
                div.className = 'document-item' + (doc.id === selectedDocument ? ' selected' : '');
                div.onclick = () => selectDocument(doc.id);
                div.innerHTML = '<strong>' + doc.name + '</strong>' +
                    '<div style="font-size: 0.9em; color: #666; margin-top: 4px; max-height: 50px; overflow: hidden;">' + 
                    doc.content.substring(0, 150) + (doc.content.length > 150 ? '...' : '') + '</div>' +
                    '<div class="item-actions">' +
                    '<button onclick="event.stopPropagation(); editDocument(\'' + doc.id + '\')">Edit</button>' +
                    '<button class="delete" onclick="event.stopPropagation(); deleteDocument(\'' + doc.id + '\')">Delete</button></div>';
                container.appendChild(div);
            });
        }

        function updateTaskSelector() {
            const select = document.getElementById('taskSelector');
            select.innerHTML = '<option value="">Select a task...</option>';
            tasks.forEach(task => {
                const option = document.createElement('option');
                option.value = task.id;
                option.textContent = task.name;
                select.appendChild(option);
            });
        }

        function updateDocumentSelector() {
            const select = document.getElementById('documentSelector');
            select.innerHTML = '<option value="">Select a document...</option>';
            documents.forEach(doc => {
                const option = document.createElement('option');
                option.value = doc.id;
                option.textContent = doc.name;
                select.appendChild(option);
            });
        }

        function selectTask(taskId) {
            selectedTask = taskId;
            document.getElementById('taskSelector').value = taskId;
            updateTasksList();
        }

        function selectDocument(docId) {
            selectedDocument = docId;
            document.getElementById('documentSelector').value = docId;
            updateDocumentsList();
        }

        function updateDiscoveredModel(modelData) {
            const key = modelData.model_name;
            const lastSeen = new Date();
            
            discoveredModels.set(key, {
                ...modelData,
                lastSeen: lastSeen,
                isOnline: modelData.status === 'online'
            });
            
            updateModelCheckboxes();
        }

        function updateDiscoveredModels(services) {
            // Handle array of services from inference monitor
            if (Array.isArray(services)) {
                services.forEach(service => {
                    updateDiscoveredModel(service);
                });
            }
        }

        function updateModelCheckboxes() {
            const container = document.getElementById('modelCheckboxes');
            
            // Save current selections
            const currentSelections = new Set();
            container.querySelectorAll('input[type="checkbox"]:checked').forEach(cb => {
                currentSelections.add(cb.value);
            });
            
            const models = Array.from(discoveredModels.values())
                .filter(model => model.model_name && model.model_name.length > 0 && model.capabilities && 
                    (model.capabilities.includes('text-generation') || model.capabilities.includes('text')))
                .sort((a, b) => {
                    if (a.isOnline !== b.isOnline) return b.isOnline - a.isOnline;
                    return a.model_name.localeCompare(b.model_name);
                });

            if (models.length === 0) {
                container.innerHTML = '<div>No text generation models found</div>';
                return;
            }

            container.innerHTML = '';
            models.forEach(model => {
                const div = document.createElement('div');
                div.className = 'model-option';
                
                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.id = 'model-' + model.model_name;
                checkbox.value = model.nats_topic;
                
                // Restore previous selection state
                if (currentSelections.has(model.nats_topic)) {
                    checkbox.checked = true;
                }
                
                const dot = document.createElement('span');
                dot.className = 'status-dot ' + (model.isOnline ? 'status-online' : 'status-offline');
                
                const label = document.createElement('label');
                label.htmlFor = checkbox.id;
                label.style.margin = '0';
                label.style.fontWeight = 'normal';
                label.style.cursor = 'pointer';
                label.textContent = model.model_name;
                
                div.appendChild(checkbox);
                div.appendChild(dot);
                div.appendChild(label);
                container.appendChild(div);
            });
        }

        async function runComparison() {
            const selectedModels = Array.from(document.querySelectorAll('#modelCheckboxes input[type="checkbox"]:checked'))
                .map(cb => ({ topic: cb.value, name: cb.id.replace('model-', '') }));
            
            if (selectedModels.length === 0 || selectedModels.length > 5) {
                alert('Please select 1-5 models');
                return;
            }

            const taskId = document.getElementById('taskSelector').value;
            const docId = document.getElementById('documentSelector').value;

            if (!taskId || !docId) {
                alert('Please select both a task and a document');
                return;
            }

            const task = tasks.find(t => t.id === taskId);
            const doc = documents.find(d => d.id === docId);
            const maxTokens = parseInt(document.getElementById('maxTokens').value);
            const temperature = parseFloat(document.getElementById('temperature').value);

            const compareButton = document.getElementById('compareButton');
            const status = document.getElementById('status');
            
            compareButton.disabled = true;
            status.style.display = 'block';
            status.textContent = 'Running comparison across ' + selectedModels.length + ' models...';

            // Create comparison grid
            const resultsContainer = document.getElementById('comparisonResults');
            resultsContainer.innerHTML = '';
            selectedModels.forEach(model => {
                const div = document.createElement('div');
                div.className = 'model-result';
                div.id = 'result-' + model.name;
                div.innerHTML = '<div class="model-header"><div class="model-name">' + model.name + '</div><div class="loading-spinner"></div></div><div class="model-response">Processing...</div>';
                resultsContainer.appendChild(div);
            });

            // Send requests to all models in parallel
            const promises = selectedModels.map(model => 
                sendModelRequest(model, task.content + doc.content, maxTokens, temperature)
            );

            try {
                const results = await Promise.allSettled(promises);
                
                results.forEach((result, index) => {
                    const model = selectedModels[index];
                    const resultDiv = document.getElementById('result-' + model.name);
                    
                    if (result.status === 'fulfilled' && result.value) {
                        const response = result.value;
                        resultDiv.className = 'model-result';
                        
                        const responseContent = response.response || response.error;
                        const renderedContent = renderMarkdown(responseContent);
                        
                        resultDiv.innerHTML = '<div class="model-header"><div class="model-name">' + model.name + '</div><div class="model-metrics">' +
                            response.tokensIn + '→' + response.tokensOut + ' tokens | ' + response.durationMs + 'ms</div></div>' +
                            '<div class="model-response">' + renderedContent + '</div>';
                    } else {
                        resultDiv.className = 'model-result error';
                        resultDiv.innerHTML = '<div class="model-header"><div class="model-name">' + model.name + '</div></div>' +
                            '<div class="model-response">Error: ' + escapeHtml(result.reason || 'Request failed') + '</div>';
                    }
                });
            } catch (error) {
                console.error('Comparison error:', error);
            } finally {
                compareButton.disabled = false;
                status.style.display = 'none';
            }
        }

        async function sendModelRequest(model, input, maxTokens, temperature) {
            try {
                const response = await fetch('/compare', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        models: [model.topic],
                        task: input,
                        document: '',
                        maxTokens: maxTokens,
                        temperature: temperature
                    })
                });

                const result = await response.json();
                
                if (result.results && result.results.length > 0) {
                    const modelResult = result.results[0];
                    return {
                        modelName: model.name,
                        response: modelResult.response,
                        tokensIn: modelResult.tokensIn,
                        tokensOut: modelResult.tokensOut,
                        durationMs: modelResult.durationMs,
                        error: modelResult.error
                    };
                } else {
                    throw new Error('No results returned');
                }
            } catch (error) {
                throw new Error(error.message);
            }
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function renderMarkdown(text) {
            let html = escapeHtml(text);
            
            // Convert line breaks
            html = html.replace(/\\n/g, '<br>');
            
            // Code blocks - triple backticks using string constructor  
            const tripleBacktick = String.fromCharCode(96) + String.fromCharCode(96) + String.fromCharCode(96);
            const codeBlockRegex = new RegExp(tripleBacktick + '([\\\\s\\\\S]*?)' + tripleBacktick, 'g');
            html = html.replace(codeBlockRegex, '<pre><code>$1</code></pre>');
            
            // Inline code - single backticks using string constructor
            const backtickRegex = new RegExp(String.fromCharCode(96) + '([^' + String.fromCharCode(96) + ']+)' + String.fromCharCode(96), 'g');
            html = html.replace(backtickRegex, '<code>$1</code>');
            
            // Bold - double asterisks
            const boldRegex = new RegExp('\\\\*\\\\*([^*]+)\\\\*\\\\*', 'g');
            html = html.replace(boldRegex, '<strong>$1</strong>');
            
            // Headers
            html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
            html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
            html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
            
            // Lists
            html = html.replace(/^- (.+)$/gm, '<li>$1</li>');
            
            return html;
        }

        // Initialize on page load
        document.addEventListener('DOMContentLoaded', function() {
            updateTaskSelector();
            updateDocumentSelector();
            updateTasksList();
            updateDocumentsList();
            
            // Connect to model discovery
            connectToModelDiscovery();

            // Event listeners
            document.getElementById('taskSelector').addEventListener('change', function() {
                selectedTask = this.value;
                updateTasksList();
            });

            document.getElementById('documentSelector').addEventListener('change', function() {
                selectedDocument = this.value;
                updateDocumentsList();
            });
        });

        async function connectToModelDiscovery() {
            try {
                const eventSource = new EventSource('/models/stream');
                
                eventSource.onmessage = function(event) {
                    const services = JSON.parse(event.data);
                    updateDiscoveredModels(services);
                };
                
                eventSource.onerror = function(error) {
                    console.log('Model discovery error:', error);
                    setTimeout(pollModels, 5000);
                };
            } catch (error) {
                console.log('Failed to connect to model discovery');
                pollModels();
            }
        }

        async function pollModels() {
            try {
                const response = await fetch('/models');
                const models = await response.json();
                models.forEach(model => updateDiscoveredModel(model));
            } catch (error) {
                console.log('Failed to poll models:', error);
            }
            setTimeout(pollModels, 10000);
        }
    </script>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, nil)
}

func (s *ComparisonServer) handleModels(w http.ResponseWriter, r *http.Request) {
	// Proxy to inference monitor
	resp, err := http.Get("http://localhost:5780/api/events")
	if err != nil {
		http.Error(w, "Failed to connect to inference monitor", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[]")) // Placeholder - real data comes from SSE
}

func (s *ComparisonServer) handleModelsStream(w http.ResponseWriter, r *http.Request) {
	// Proxy to inference monitor's SSE endpoint
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Forward SSE from inference monitor
	resp, err := http.Get("http://localhost:5780/api/events")
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\": \"Failed to connect to inference monitor\"}\n\n")
		return
	}
	defer resp.Body.Close()

	// Copy the SSE stream
	buffer := make([]byte, 4096)
	for {
		select {
		case <-r.Context().Done():
			return
		default:
			n, err := resp.Body.Read(buffer)
			if err != nil {
				return
			}
			if n > 0 {
				w.Write(buffer[:n])
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	}
}

func (s *ComparisonServer) handleComparison(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req ComparisonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Models) == 0 || len(req.Models) > 5 {
		http.Error(w, "Must specify 1-5 models", http.StatusBadRequest)
		return
	}

	// Run inference on all models in parallel
	var wg sync.WaitGroup
	results := make([]ModelResult, len(req.Models))
	
	for i, modelSubject := range req.Models {
		wg.Add(1)
		go func(index int, subject string) {
			defer wg.Done()
			results[index] = s.runSingleInference(subject, req.Task+req.Document, req.MaxTokens, req.Temperature)
		}(i, modelSubject)
	}

	wg.Wait()

	response := ComparisonResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *ComparisonServer) runSingleInference(subject, input string, maxTokens int, temperature float64) ModelResult {
	replySubject := fmt.Sprintf("comparison.reply.%d", time.Now().UnixNano())
	
	natsReq := InferenceRequest{
		ReqID:   fmt.Sprintf("comparison-%d", time.Now().UnixNano()),
		Input:   input,
		ReplyTo: replySubject,
		Raw:     false,
		Params: map[string]interface{}{
			"max_tokens":  maxTokens,
			"temperature": temperature,
		},
	}

	// Subscribe to reply
	replyChan := make(chan *nats.Msg, 1)
	sub, err := s.natsConn.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		return ModelResult{Error: fmt.Sprintf("Failed to subscribe: %v", err)}
	}
	defer sub.Unsubscribe()

	// Send request
	reqBytes, _ := json.Marshal(natsReq)
	if err := s.natsConn.Publish(subject, reqBytes); err != nil {
		return ModelResult{Error: fmt.Sprintf("Failed to publish: %v", err)}
	}

	// Wait for response
	select {
	case msg := <-replyChan:
		var response InferenceResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			return ModelResult{Error: fmt.Sprintf("Failed to parse response: %v", err)}
		}
		
		return ModelResult{
			ModelName:  subject,
			Response:   response.Text,
			TokensIn:   response.TokensIn,
			TokensOut:  response.TokensOut,
			DurationMs: response.DurationMs,
			Error:      response.Error,
		}
		
	case <-time.After(2 * time.Minute):
		return ModelResult{Error: "Request timeout"}
	}
}

func (s *ComparisonServer) Close() {
	if s.natsConn != nil {
		s.natsConn.Close()
	}
}

func main() {
	var (
		natsURL = flag.String("nats", "nats://127.0.0.1:5700", "NATS server URL")
		httpAddr = flag.String("addr", ":8878", "HTTP server address")
	)
	flag.Parse()

	server, err := NewComparisonServer(*natsURL)
	if err != nil {
		log.Fatalf("Failed to create comparison server: %v", err)
	}
	defer server.Close()

	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/compare", server.handleComparison)
	http.HandleFunc("/models", server.handleModels)
	http.HandleFunc("/models/stream", server.handleModelsStream)

	fmt.Printf("🔍 Starting Model Comparison Chat on http://localhost%s\n", *httpAddr)
	fmt.Printf("📡 Connected to NATS: %s\n", *natsURL)
	fmt.Println("Open your browser and start comparing models!")

	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}