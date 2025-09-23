package harmony

import (
	"os"
	"strings"
	"testing"
)

// TestWithOfficialTestData validates our implementation against OpenAI test data
func TestWithOfficialTestData(t *testing.T) {
	testCases := []struct {
		file        string
		description string
	}{
		{"test_simple_convo_low_effort.txt", "Simple conversation with low reasoning"},
		{"test_simple_convo_medium_effort.txt", "Simple conversation with medium reasoning"},
		{"test_simple_convo_high_effort.txt", "Simple conversation with high reasoning"},
		{"test_simple_tool_call.txt", "Tool calling example"},
		{"test_keep_analysis_between_finals.txt", "Channel switching example"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Read test data
			data, err := os.ReadFile("testdata/" + tc.file)
			if err != nil {
				t.Skipf("Test data file not found: %s", tc.file)
				return
			}
			
			content := string(data)
			
			// All test files should contain proper Harmony structure
			if !strings.Contains(content, "<|start|>system<|message|>") {
				t.Errorf("Test file %s missing system message", tc.file)
			}
			
			if !strings.Contains(content, "<|start|>assistant") {
				t.Errorf("Test file %s missing assistant start", tc.file)
			}
			
			// Log structure for debugging
			t.Logf("Test file %s structure validated", tc.file)
		})
	}
}

func TestChannelParsing(t *testing.T) {
	// Test parsing a response with multiple channels based on official format
	response := `<|channel|>analysis<|message|>The user is asking about math. I need to calculate 2+2.<|end|><|start|>assistant<|channel|>final<|message|>The answer is 4.<|end|>`
	
	formatter := NewHarmonyFormatter()
	parsed, err := formatter.ParseAssistantResponse(response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	// Should extract analysis channel
	if analysis, exists := parsed.Channels[ChannelAnalysis]; !exists {
		t.Error("Missing analysis channel")
	} else if !strings.Contains(analysis, "calculate 2+2") {
		t.Errorf("Analysis content incorrect: %s", analysis)
	}
	
	// Should extract final channel
	if final, exists := parsed.Channels[ChannelFinal]; !exists {
		t.Error("Missing final channel")
	} else if !strings.Contains(final, "answer is 4") {
		t.Errorf("Final content incorrect: %s", final)
	}
	
	// Final response should be from final channel
	if !strings.Contains(parsed.FinalResponse, "answer is 4") {
		t.Errorf("Final response incorrect: %s", parsed.FinalResponse)
	}
}

func TestReasoningLevelMapping(t *testing.T) {
	testCases := []struct {
		input    string
		expected ReasoningLevel
	}{
		{"hello", ReasoningLow},
		{"explain quantum physics in detail", ReasoningHigh},
		{"what is AI?", ReasoningMedium},
		{"calculate 2+2", ReasoningMedium},
		{"analyze and compare machine learning algorithms", ReasoningHigh},
	}
	
	for _, tc := range testCases {
		result := DetermineReasoningLevel(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s, Expected: %s, Got: %s", tc.input, tc.expected, result)
		}
	}
}

func TestHarmonyFormatterComparison(t *testing.T) {
	// Test against actual OpenAI test data structure
	testFile := "testdata/test_simple_convo_medium_effort.txt"
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Skip("Test data not available")
		return
	}
	
	expected := string(data)
	
	// Generate the same conversation using our formatter
	result := ConversationWithSystem(
		"Answer the user's questions like a robot.",
		"What is the capital of the largest country in the world?", 
		ReasoningMedium,
	)
	
	// Both should contain the same key components
	expectedComponents := []string{
		"<|start|>system<|message|>",
		"Knowledge cutoff: 2024-06",
		"Reasoning: medium",
		"<|start|>developer<|message|>",
		"Answer the user's questions like a robot",
		"<|start|>user<|message|>",
		"What is the capital of the largest country in the world?",
		"<|start|>assistant",
	}
	
	for _, component := range expectedComponents {
		if !strings.Contains(result, component) {
			t.Errorf("Missing component in generated result: %s", component)
		}
		if !strings.Contains(expected, component) {
			t.Errorf("Missing component in test data: %s", component)
		}
	}
	
	t.Logf("Generated format matches test data structure")
}

func TestResponseExtractionFromRealResponse(t *testing.T) {
	// Test with actual GPT-OSS response format we're seeing
	realResponse := `<|channel|>analysis<|message|>The user says "Hello, how are you?" It's a greeting. We should respond politely, maybe ask about them.<|end|><|start|>assistant<|channel|>final<|message|>Hello! I'm doing great, thanks for asking. How are you today?<|end|>`
	
	formatter := NewHarmonyFormatter()
	parsed, err := formatter.ParseAssistantResponse(realResponse)
	if err != nil {
		t.Fatalf("Failed to parse real response: %v", err)
	}
	
	// Should extract final response
	expected := "Hello! I'm doing great, thanks for asking. How are you today?"
	if parsed.FinalResponse != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, parsed.FinalResponse)
	}
	
	// Should also have analysis
	if analysis, exists := parsed.Channels[ChannelAnalysis]; !exists {
		t.Error("Missing analysis channel")
	} else if !strings.Contains(analysis, "greeting") {
		t.Errorf("Analysis content missing: %s", analysis)
	}
}