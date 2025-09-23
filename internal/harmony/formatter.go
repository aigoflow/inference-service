package harmony

import (
	"fmt"
	"strings"
	"time"
)

// HarmonyFormatter handles conversion to/from Harmony format
type HarmonyFormatter struct {
	config SystemConfig
}

// NewHarmonyFormatter creates a new Harmony formatter
func NewHarmonyFormatter() *HarmonyFormatter {
	return &HarmonyFormatter{
		config: SystemConfig{
			ModelIdentity:   "ChatGPT, a large language model trained by OpenAI",
			KnowledgeCutoff: "2024-06",
			CurrentDate:     time.Now().Format("2006-01-02"),
			ReasoningLevel:  ReasoningMedium,
			ValidChannels:   []Channel{ChannelAnalysis, ChannelCommentary, ChannelFinal},
		},
	}
}

// FormatConversationForCompletion formats a conversation for GPT-OSS completion
func (hf *HarmonyFormatter) FormatConversationForCompletion(conversation *Conversation) string {
	var prompt strings.Builder
	
	// System message with configuration
	prompt.WriteString("<|start|>system<|message|>")
	prompt.WriteString(conversation.SystemConfig.ModelIdentity)
	if !strings.HasSuffix(conversation.SystemConfig.ModelIdentity, ".") {
		prompt.WriteString(".")
	}
	prompt.WriteString("\nKnowledge cutoff: ")
	prompt.WriteString(conversation.SystemConfig.KnowledgeCutoff)
	prompt.WriteString("\n\nReasoning: ")
	prompt.WriteString(string(conversation.SystemConfig.ReasoningLevel))
	
	// Add valid channels information
	if len(conversation.SystemConfig.ValidChannels) > 0 {
		prompt.WriteString("\n\n# Valid channels: ")
		channelStrs := make([]string, len(conversation.SystemConfig.ValidChannels))
		for i, ch := range conversation.SystemConfig.ValidChannels {
			channelStrs[i] = string(ch)
		}
		prompt.WriteString(strings.Join(channelStrs, ", "))
		prompt.WriteString(". Channel must be included for every message.")
	}
	
	// Add tools if present
	if len(conversation.Tools) > 0 {
		prompt.WriteString("\n\n# Available tools:\n")
		for _, tool := range conversation.Tools {
			prompt.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		}
	}
	
	prompt.WriteString("<|end|>")
	
	// Add all messages
	for _, message := range conversation.Messages {
		prompt.WriteString(fmt.Sprintf("<|start|>%s<|message|>", string(message.Role)))
		
		// For non-system messages, add content
		if message.Role != RoleSystem {
			if message.Role == RoleDeveloper {
				prompt.WriteString("# Instructions\n\n")
			}
			prompt.WriteString(message.Content)
		}
		
		prompt.WriteString("<|end|>")
	}
	
	// Start assistant response
	prompt.WriteString("<|start|>assistant")
	
	return prompt.String()
}

// ParseAssistantResponse parses a GPT-OSS assistant response back into structured format
func (hf *HarmonyFormatter) ParseAssistantResponse(response string) (*AssistantResponse, error) {
	result := &AssistantResponse{
		Channels: make(map[Channel]string),
	}
	
	// Simple string-based parsing for the patterns we see
	// Pattern 1: <|channel|>channelname<|message|>content
	// Pattern 2: <|start|>assistant<|channel|>channelname<|message|>content<|end|>
	if strings.Contains(response, "<|channel|>final<|message|>") {
		// Extract final channel content
		start := strings.Index(response, "<|channel|>final<|message|>")
		if start != -1 {
			start += len("<|channel|>final<|message|>")
			end := strings.Index(response[start:], "<|end|>")
			if end == -1 {
				end = len(response) - start
			}
			finalContent := strings.TrimSpace(response[start : start+end])
			result.Channels[ChannelFinal] = finalContent
			result.FinalResponse = finalContent
		}
	}
	
	if strings.Contains(response, "<|channel|>analysis<|message|>") {
		// Extract analysis channel content
		start := strings.Index(response, "<|channel|>analysis<|message|>")
		if start != -1 {
			start += len("<|channel|>analysis<|message|>")
			end := strings.Index(response[start:], "<|end|>")
			if end == -1 {
				// Look for next channel or end of string
				if nextChannel := strings.Index(response[start:], "<|channel|>"); nextChannel != -1 {
					end = nextChannel
				} else {
					end = len(response) - start
				}
			}
			analysisContent := strings.TrimSpace(response[start : start+end])
			result.Channels[ChannelAnalysis] = analysisContent
			
			// If no final channel found, use analysis as final response
			if result.FinalResponse == "" {
				result.FinalResponse = analysisContent
			}
		}
	}
	
	// If neither channel found, use entire response
	if result.FinalResponse == "" {
		result.FinalResponse = response
	}
	
	return result, nil
}

// AssistantResponse represents parsed assistant response
type AssistantResponse struct {
	FinalResponse string             `json:"final_response"`
	Channels      map[Channel]string `json:"channels"`
	ToolCalls     []ToolCall         `json:"tool_calls"`
	Reasoning     string             `json:"reasoning"`
}

// ToolCall represents a function call
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	CallID    string                 `json:"call_id"`
}

// parseToolCall extracts tool call from a line
func (hf *HarmonyFormatter) parseToolCall(line string) *ToolCall {
	// Simple parser for <|call|>function_name(args)<|return|>
	if !strings.Contains(line, "<|call|>") {
		return nil
	}
	
	// Extract content between <|call|> and <|return|>
	start := strings.Index(line, "<|call|>") + 7
	end := strings.Index(line, "<|return|>")
	if end == -1 {
		end = len(line)
	}
	
	callContent := line[start:end]
	
	// Parse function call (simplified)
	if parenIdx := strings.Index(callContent, "("); parenIdx != -1 {
		funcName := strings.TrimSpace(callContent[:parenIdx])
		argsStr := callContent[parenIdx+1:]
		if endParen := strings.LastIndex(argsStr, ")"); endParen != -1 {
			argsStr = argsStr[:endParen]
		}
		
		return &ToolCall{
			Name:      funcName,
			Arguments: hf.parseArguments(argsStr),
			CallID:    fmt.Sprintf("call_%d", time.Now().UnixNano()),
		}
	}
	
	return nil
}

// parseArguments parses function arguments (simplified)
func (hf *HarmonyFormatter) parseArguments(argsStr string) map[string]interface{} {
	args := make(map[string]interface{})
	
	// Simple key=value parsing (would need more sophisticated parsing for complex args)
	pairs := strings.Split(argsStr, ",")
	for _, pair := range pairs {
		if eqIdx := strings.Index(pair, "="); eqIdx != -1 {
			key := strings.TrimSpace(pair[:eqIdx])
			value := strings.TrimSpace(pair[eqIdx+1:])
			
			// Remove quotes if present
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}
			
			args[key] = value
		}
	}
	
	return args
}

// SetSystemConfig updates the system configuration
func (hf *HarmonyFormatter) SetSystemConfig(config SystemConfig) {
	hf.config = config
}

// GetSystemConfig returns current system configuration
func (hf *HarmonyFormatter) GetSystemConfig() SystemConfig {
	return hf.config
}