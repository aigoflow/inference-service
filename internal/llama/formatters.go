package llama

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	
	"github.com/aigoflow/inference-service/internal/config"
	"github.com/aigoflow/inference-service/internal/harmony"
)

// PromptFormatter interface for different prompt formats
type PromptFormatter interface {
	FormatPrompt(input string, systemPrompt string, config map[string]interface{}) string
	ParseResponse(response string, config map[string]interface{}) string
	Name() string
}

// StandardFormatter handles basic prompts without templates
type StandardFormatter struct{}

func (f *StandardFormatter) Name() string {
	return "standard"
}

func (f *StandardFormatter) FormatPrompt(input, systemPrompt string, config map[string]interface{}) string {
	if systemPrompt != "" {
		return systemPrompt + "\n\nUser: " + input + "\nAssistant: "
	}
	return input
}

func (f *StandardFormatter) ParseResponse(response string, config map[string]interface{}) string {
	return response // No parsing needed for standard format
}

// TemplateFormatter handles the existing template system
type TemplateFormatter struct{}

func (f *TemplateFormatter) Name() string {
	return "template"
}

func (f *TemplateFormatter) FormatPrompt(input, systemPrompt string, config map[string]interface{}) string {
	// Load template file from model directory and apply it
	modelPath, ok := config["model_path"].(string)
	if !ok {
		return input
	}
	
	// Load the actual template file (prompt_template.json) from model directory
	modelDir := filepath.Dir(modelPath)
	template, err := loadTemplate(modelDir)
	if err != nil {
		slog.Warn("Template load failed, using passthrough", "error", err)
		return input
	}
	
	if template == nil {
		// No template file = passthrough mode
		return input
	}
	
	// Apply template formatting using the loaded template
	var formatted strings.Builder
	
	if template.SystemRole != "" {
		formatted.WriteString(template.SystemRole)
		formatted.WriteString("\n")
	}
	
	formatted.WriteString(template.UserPrefix)
	formatted.WriteString(input)
	formatted.WriteString(template.UserSuffix)
	formatted.WriteString(template.ModelPrefix)
	
	return formatted.String()
}

func (f *TemplateFormatter) ParseResponse(response string, config map[string]interface{}) string {
	return response // No parsing needed for template format
}

// HarmonyFormatter handles GPT-OSS Harmony format
type HarmonyFormatter struct{}

func (f *HarmonyFormatter) Name() string {
	return "harmony"
}

func (f *HarmonyFormatter) FormatPrompt(input, systemPrompt string, config map[string]interface{}) string {
	// Get reasoning level from config
	reasoningLevel := harmony.ReasoningMedium
	if level, ok := config["reasoning_level"].(string); ok {
		switch level {
		case "low":
			reasoningLevel = harmony.ReasoningLow
		case "high":
			reasoningLevel = harmony.ReasoningHigh
		case "medium":
			reasoningLevel = harmony.ReasoningMedium
		}
	}
	
	// Build conversation with proper configuration
	builder := harmony.NewConversationBuilder().
		WithReasoningLevel(reasoningLevel)
	
	// Set model identity if configured
	if identity, ok := config["model_identity"].(string); ok {
		builder = builder.WithModelIdentity(identity)
	}
	
	// Add system prompt as developer message if provided
	if systemPrompt != "" {
		builder = builder.AddDeveloperMessage(systemPrompt)
	}
	
	// Add user message
	builder = builder.AddUserMessage(input)
	
	// Format for completion
	conversation := builder.Build()
	formatter := harmony.NewHarmonyFormatter()
	
	return formatter.FormatConversationForCompletion(conversation)
}

func (f *HarmonyFormatter) ParseResponse(response string, config map[string]interface{}) string {
	// Check if we should extract final response
	extractFinal := true
	if extract, ok := config["extract_final"].(bool); ok {
		extractFinal = extract
	}
	
	if !extractFinal {
		return response // Return full response
	}
	
	// Extract final user-facing response
	return harmony.ExtractFinalResponse(response)
}

// ChatMLFormatter handles ChatML format (example for extensibility)
type ChatMLFormatter struct{}

func (f *ChatMLFormatter) Name() string {
	return "chatml"
}

func (f *ChatMLFormatter) FormatPrompt(input, systemPrompt string, config map[string]interface{}) string {
	var prompt strings.Builder
	
	if systemPrompt != "" {
		prompt.WriteString("<|im_start|>system\n")
		prompt.WriteString(systemPrompt)
		prompt.WriteString("<|im_end|>\n")
	}
	
	prompt.WriteString("<|im_start|>user\n")
	prompt.WriteString(input)
	prompt.WriteString("<|im_end|>\n")
	prompt.WriteString("<|im_start|>assistant\n")
	
	return prompt.String()
}

func (f *ChatMLFormatter) ParseResponse(response string, config map[string]interface{}) string {
	// Remove ChatML end tokens if present
	response = strings.TrimSuffix(response, "<|im_end|>")
	return strings.TrimSpace(response)
}

// FormatterRegistry manages available formatters
type FormatterRegistry struct {
	formatters map[string]PromptFormatter
}

func NewFormatterRegistry() *FormatterRegistry {
	registry := &FormatterRegistry{
		formatters: make(map[string]PromptFormatter),
	}
	
	// Register built-in formatters
	registry.Register(&StandardFormatter{})
	registry.Register(&TemplateFormatter{})
	registry.Register(&HarmonyFormatter{})
	registry.Register(&ChatMLFormatter{})
	
	return registry
}

func (r *FormatterRegistry) Register(formatter PromptFormatter) {
	r.formatters[formatter.Name()] = formatter
}

func (r *FormatterRegistry) GetFormatter(name string) (PromptFormatter, error) {
	if formatter, exists := r.formatters[name]; exists {
		return formatter, nil
	}
	return nil, fmt.Errorf("formatter '%s' not found", name)
}

func (r *FormatterRegistry) ListFormatters() []string {
	names := make([]string, 0, len(r.formatters))
	for name := range r.formatters {
		names = append(names, name)
	}
	return names
}

// Global formatter registry
var globalFormatterRegistry = NewFormatterRegistry()

// FormatPromptWithConfig formats prompt using configuration-driven approach
func FormatPromptWithConfig(input, modelPath string, cfg *config.Config) string {
	if cfg == nil {
		slog.Warn("No configuration provided, using passthrough")
		return input
	}
	
	// Get formatter based on configuration
	formatter, err := globalFormatterRegistry.GetFormatter(cfg.ModelFormat)
	if err != nil {
		slog.Warn("Unknown format, falling back to standard", 
			"format", cfg.ModelFormat, 
			"available", globalFormatterRegistry.ListFormatters())
		formatter, _ = globalFormatterRegistry.GetFormatter("standard")
	}
	
	// Load system prompt from template if available
	systemPrompt := ""
	if template, err := loadTemplate(filepath.Dir(modelPath)); err == nil && template != nil {
		systemPrompt = template.SystemRole
	}
	
	// Add model_path to config for formatters that need it
	formatConfig := make(map[string]interface{})
	for k, v := range cfg.FormatConfig {
		formatConfig[k] = v
	}
	formatConfig["model_path"] = modelPath
	
	slog.Info("Formatting prompt", 
		"formatter", formatter.Name(), 
		"model_path", modelPath,
		"has_system_prompt", systemPrompt != "")
	
	return formatter.FormatPrompt(input, systemPrompt, formatConfig)
}

// ParseResponseWithConfig parses response using configuration-driven approach  
func ParseResponseWithConfig(response, modelPath string, cfg *config.Config) string {
	if cfg == nil {
		return response
	}
	
	// Get formatter based on configuration
	formatter, err := globalFormatterRegistry.GetFormatter(cfg.ModelFormat)
	if err != nil {
		return response // No parsing if formatter not found
	}
	
	parsed := formatter.ParseResponse(response, cfg.FormatConfig)
	
	slog.Debug("Parsed response", 
		"formatter", formatter.Name(),
		"original_length", len(response),
		"parsed_length", len(parsed))
	
	return parsed
}