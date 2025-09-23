package llama

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/aigoflow/inference-service/internal/harmony"
)

type PromptTemplate struct {
	Name        string `json:"name"`
	SystemRole  string `json:"system_role,omitempty"`
	UserPrefix  string `json:"user_prefix"`
	UserSuffix  string `json:"user_suffix"`
	ModelPrefix string `json:"model_prefix"`
	ModelSuffix string `json:"model_suffix,omitempty"`
}

func loadTemplate(modelDir string) (*PromptTemplate, error) {
	templatePath := filepath.Join(modelDir, "prompt_template.json")
	
	// Check if template exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		slog.Debug("No custom template found, using passthrough", "path", templatePath)
		return nil, nil // No template = passthrough mode
	}
	
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %v", err)
	}
	
	var template PromptTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}
	
	slog.Info("Loaded prompt template", "name", template.Name, "path", templatePath)
	return &template, nil
}

func formatPromptForModel(input, modelPath string) string {
	// Check if this is a GPT-OSS model that needs Harmony format
	if harmony.IsGPTOSSModel(modelPath) {
		slog.Info("Using Harmony format for GPT-OSS model", "model_path", modelPath)
		
		// Extract system prompt from template if available
		modelDir := filepath.Dir(modelPath)
		template, err := loadTemplate(modelDir)
		if err == nil && template != nil && template.SystemRole != "" {
			// Use system prompt with Harmony format
			return harmony.FormatPromptForGPTOSSWithSystem(input, template.SystemRole, modelPath)
		}
		
		// Use simple Harmony format without system prompt
		return harmony.FormatPromptForGPTOSS(input, modelPath)
	}
	
	// For non-GPT-OSS models, use existing template system
	modelDir := filepath.Dir(modelPath)
	template, err := loadTemplate(modelDir)
	if err != nil {
		slog.Warn("Template load failed, using passthrough", "error", err)
		return input
	}
	
	if template == nil {
		// No template = passthrough mode
		return input
	}
	
	// Apply template formatting
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

type GrammarConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Grammar     string `json:"grammar"`
}

func loadGrammar(modelPath, grammarName string) (string, error) {
	modelDir := filepath.Dir(modelPath)
	grammarsPath := filepath.Join(modelDir, "grammars.json")
	
	// Check if grammars config exists
	if _, err := os.Stat(grammarsPath); os.IsNotExist(err) {
		return "", fmt.Errorf("no grammars.json found in %s", modelDir)
	}
	
	data, err := os.ReadFile(grammarsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read grammars config: %v", err)
	}
	
	var grammars map[string]GrammarConfig
	if err := json.Unmarshal(data, &grammars); err != nil {
		return "", fmt.Errorf("failed to parse grammars config: %v", err)
	}
	
	grammar, exists := grammars[grammarName]
	if !exists {
		available := make([]string, 0, len(grammars))
		for name := range grammars {
			available = append(available, name)
		}
		return "", fmt.Errorf("grammar '%s' not found. Available: %v", grammarName, available)
	}
	
	slog.Info("Loaded grammar", "name", grammar.Name, "description", grammar.Description)
	return grammar.Grammar, nil
}

func resolveGrammar(modelPath string, grammarParam interface{}) string {
	if grammarParam == nil {
		return ""
	}
	
	grammarStr, ok := grammarParam.(string)
	if !ok {
		return ""
	}
	
	// If it starts with standard grammar syntax, use directly
	if strings.Contains(grammarStr, "::=") {
		slog.Debug("Using inline grammar")
		return grammarStr
	}
	
	// Otherwise, try to load as named grammar
	loadedGrammar, err := loadGrammar(modelPath, grammarStr)
	if err != nil {
		slog.Warn("Failed to load named grammar, treating as inline", "name", grammarStr, "error", err)
		return grammarStr // Fallback to treat as inline grammar
	}
	
	return loadedGrammar
}