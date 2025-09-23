package harmony

import (
	"strings"
	"testing"
)

func TestSimpleConversation(t *testing.T) {
	result := SimpleConversation("What is the capital of the largest country in the world?", ReasoningLow)
	
	// Should contain system message
	if !strings.Contains(result, "<|start|>system<|message|>") {
		t.Error("Missing system message")
	}
	
	// Should contain reasoning level
	if !strings.Contains(result, "Reasoning: low") {
		t.Error("Missing reasoning level")
	}
	
	// Should contain user message
	if !strings.Contains(result, "<|start|>user<|message|>") {
		t.Error("Missing user message")
	}
	
	// Should contain the user input
	if !strings.Contains(result, "What is the capital of the largest country in the world?") {
		t.Error("Missing user input")
	}
	
	// Should end with assistant start
	if !strings.HasSuffix(result, "<|start|>assistant") {
		t.Error("Should end with assistant start tag")
	}
}

func TestConversationWithSystem(t *testing.T) {
	result := ConversationWithSystem(
		"Answer the user's questions like a robot.",
		"What is AI?",
		ReasoningMedium,
	)
	
	// Should contain developer message
	if !strings.Contains(result, "<|start|>developer<|message|>") {
		t.Error("Missing developer message")
	}
	
	// Should contain instructions header
	if !strings.Contains(result, "# Instructions") {
		t.Error("Missing instructions header")
	}
	
	// Should contain the system prompt
	if !strings.Contains(result, "Answer the user's questions like a robot.") {
		t.Error("Missing system prompt")
	}
	
	// Should contain medium reasoning
	if !strings.Contains(result, "Reasoning: medium") {
		t.Error("Missing medium reasoning level")
	}
}

func TestConversationWithTools(t *testing.T) {
	tools := []Tool{
		{
			Name:        "calculator",
			Description: "Perform mathematical calculations",
			Parameters:  map[string]interface{}{"expression": "string"},
		},
	}
	
	result := ConversationWithTools("Calculate 2 + 2", tools, ReasoningHigh)
	
	// Should contain tools section
	if !strings.Contains(result, "# Available tools:") {
		t.Error("Missing tools section")
	}
	
	// Should contain tool definition
	if !strings.Contains(result, "calculator: Perform mathematical calculations") {
		t.Error("Missing tool definition")
	}
	
	// Should contain high reasoning
	if !strings.Contains(result, "Reasoning: high") {
		t.Error("Missing high reasoning level")
	}
}

func TestParseAssistantResponse(t *testing.T) {
	// Test parsing a response with thinking and final answer
	response := `<think>
The user is asking about AI. I should provide a clear explanation.
</think>

AI stands for Artificial Intelligence. It refers to computer systems that can perform tasks that typically require human intelligence.`
	
	formatter := NewHarmonyFormatter()
	parsed, err := formatter.ParseAssistantResponse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	// Should extract thinking as analysis
	if analysis, exists := parsed.Channels[ChannelAnalysis]; !exists {
		t.Error("Missing analysis channel")
	} else if !strings.Contains(analysis, "clear explanation") {
		t.Error("Analysis content not extracted correctly")
	}
	
	// Should extract final response
	if !strings.Contains(parsed.FinalResponse, "AI stands for Artificial Intelligence") {
		t.Error("Final response not extracted correctly")
	}
}

func TestToolCallParsing(t *testing.T) {
	response := `I'll calculate that for you.

<|call|>calculator(expression="2 + 2")<|return|>

The result is 4.`
	
	formatter := NewHarmonyFormatter()
	parsed, err := formatter.ParseAssistantResponse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	// Should extract tool call
	if len(parsed.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(parsed.ToolCalls))
	}
	
	toolCall := parsed.ToolCalls[0]
	if toolCall.Name != "calculator" {
		t.Errorf("Expected tool name 'calculator', got '%s'", toolCall.Name)
	}
	
	if expr, exists := toolCall.Arguments["expression"]; !exists {
		t.Error("Missing expression argument")
	} else if expr != "2 + 2" {
		t.Errorf("Expected expression '2 + 2', got '%v'", expr)
	}
}

func TestFormatterMatchesTestData(t *testing.T) {
	// This should match the official test data format
	expected := `<|start|>system<|message|>ChatGPT, a large language model trained by OpenAI.
Knowledge cutoff: 2024-06

Reasoning: low

# Valid channels: analysis, commentary, final. Channel must be included for every message.<|end|><|start|>developer<|message|># Instructions

Answer the user's questions like a robot.<|end|><|start|>user<|message|>What is the capital of the largest country in the world?<|end|><|start|>assistant`
	
	result := ConversationWithSystem(
		"Answer the user's questions like a robot.",
		"What is the capital of the largest country in the world?",
		ReasoningLow,
	)
	
	// Remove dynamic date for comparison (use current date)
	formatter := NewHarmonyFormatter()
	result = strings.ReplaceAll(result, "Current date: "+formatter.config.CurrentDate, "")
	expected = strings.ReplaceAll(expected, "Current date: ", "")
	
	if strings.TrimSpace(result) != strings.TrimSpace(expected) {
		t.Errorf("Format doesn't match expected test data.\nGot:\n%s\n\nExpected:\n%s", result, expected)
	}
}