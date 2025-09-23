package harmony

// SimpleConversation creates a basic user/assistant conversation
func SimpleConversation(userInput string, reasoning ReasoningLevel) string {
	conversation := NewConversationBuilder().
		WithReasoningLevel(reasoning).
		AddUserMessage(userInput).
		Build()
	
	formatter := NewHarmonyFormatter()
	return formatter.FormatConversationForCompletion(conversation)
}

// ConversationWithSystem creates a conversation with system prompt
func ConversationWithSystem(systemPrompt, userInput string, reasoning ReasoningLevel) string {
	conversation := NewConversationBuilder().
		WithReasoningLevel(reasoning).
		AddDeveloperMessage(systemPrompt).
		AddUserMessage(userInput).
		Build()
	
	formatter := NewHarmonyFormatter()
	return formatter.FormatConversationForCompletion(conversation)
}

// ConversationWithTools creates a conversation with tool support
func ConversationWithTools(userInput string, tools []Tool, reasoning ReasoningLevel) string {
	conversation := NewConversationBuilder().
		WithReasoningLevel(reasoning).
		WithTools(tools).
		AddUserMessage(userInput).
		Build()
	
	formatter := NewHarmonyFormatter()
	return formatter.FormatConversationForCompletion(conversation)
}

// FullConversation creates a complete conversation with all components
func FullConversation(systemPrompt, userInput string, tools []Tool, reasoning ReasoningLevel) string {
	conversation := NewConversationBuilder().
		WithReasoningLevel(reasoning).
		WithTools(tools).
		AddDeveloperMessage(systemPrompt).
		AddUserMessage(userInput).
		Build()
	
	formatter := NewHarmonyFormatter()
	return formatter.FormatConversationForCompletion(conversation)
}

// ParseResponse parses a GPT-OSS response into structured format
func ParseResponse(response string) (*AssistantResponse, error) {
	formatter := NewHarmonyFormatter()
	return formatter.ParseAssistantResponse(response)
}