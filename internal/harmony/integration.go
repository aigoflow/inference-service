package harmony

import (
	"strings"
)

// IsGPTOSSModel checks if a model path indicates a GPT-OSS model
func IsGPTOSSModel(modelPath string) bool {
	modelPath = strings.ToLower(modelPath)
	return strings.Contains(modelPath, "gpt-oss") || 
		   strings.Contains(modelPath, "gpt_oss")
}

// FormatPromptForGPTOSS formats a simple prompt using Harmony format
func FormatPromptForGPTOSS(input, modelPath string) string {
	if !IsGPTOSSModel(modelPath) {
		// Not a GPT-OSS model, return input as-is
		return input
	}
	
	// Determine reasoning level based on input complexity
	reasoning := DetermineReasoningLevel(input)
	
	// Use simple conversation format
	return SimpleConversation(input, reasoning)
}

// FormatPromptForGPTOSSWithSystem formats with system prompt
func FormatPromptForGPTOSSWithSystem(input, systemPrompt, modelPath string) string {
	if !IsGPTOSSModel(modelPath) {
		// Not a GPT-OSS model, use simple format
		return systemPrompt + "\n\nUser: " + input + "\nAssistant: "
	}
	
	reasoning := DetermineReasoningLevel(input)
	return ConversationWithSystem(systemPrompt, input, reasoning)
}

// DetermineReasoningLevel automatically determines reasoning level based on input
func DetermineReasoningLevel(input string) ReasoningLevel {
	input = strings.ToLower(input)
	
	// High reasoning keywords
	highReasoningKeywords := []string{
		"explain", "analyze", "compare", "evaluate", "detailed", "comprehensive",
		"step by step", "reasoning", "logic", "proof", "algorithm", "strategy",
		"complex", "intricate", "sophisticated", "elaborate", "thorough",
	}
	
	// Low reasoning keywords  
	lowReasoningKeywords := []string{
		"hello", "hi", "thanks", "thank you", "yes", "no", "ok", "okay",
		"simple", "quick", "brief", "short", "what is", "who is",
	}
	
	// Check for high reasoning indicators
	for _, keyword := range highReasoningKeywords {
		if strings.Contains(input, keyword) {
			return ReasoningHigh
		}
	}
	
	// Check for low reasoning indicators
	for _, keyword := range lowReasoningKeywords {
		if strings.Contains(input, keyword) {
			return ReasoningLow
		}
	}
	
	// Default to medium for everything else
	return ReasoningMedium
}

// ExtractFinalResponse extracts the main response from GPT-OSS output
func ExtractFinalResponse(gptossResponse string) string {
	formatter := NewHarmonyFormatter()
	parsed, err := formatter.ParseAssistantResponse(gptossResponse)
	if err != nil {
		// Fallback: return original response
		return gptossResponse
	}
	
	if parsed.FinalResponse != "" {
		return parsed.FinalResponse
	}
	
	// Try to extract from final channel
	if final, exists := parsed.Channels[ChannelFinal]; exists && final != "" {
		return final
	}
	
	// Fallback: return original
	return gptossResponse
}