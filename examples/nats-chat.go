package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type InferenceRequest struct {
	TraceID string                 `json:"trace_id,omitempty"`
	ReqID   string                 `json:"req_id"`
	Input   string                 `json:"input"`
	Params  map[string]interface{} `json:"params"`
	ReplyTo string                 `json:"reply_to,omitempty"`
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

// startSpinner shows a rotating animation while waiting for response
func startSpinner(ctx context.Context, message string) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	
	go func() {
		defer wg.Done()
		
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				// Clear the spinner line more gently
				fmt.Printf("\r%s\r", strings.Repeat(" ", len(message)+10))
				return
			case <-ticker.C:
				fmt.Printf("\r%s %s", frames[i], message)
				i = (i + 1) % len(frames)
			}
		}
	}()
	
	return &wg
}

func main() {
	var (
		natsURL     = flag.String("url", "nats://127.0.0.1:4222", "NATS server URL")
		subject     = flag.String("subject", "inference.request.gemma3-270m", "NATS subject to publish to")
		input       = flag.String("input", "", "Text input for inference (one-shot mode)")
		maxTokens   = flag.Int("max-tokens", -1, "Maximum tokens to generate (-1 for maximum possible)")
		temperature = flag.Float64("temperature", 0.7, "Temperature for generation")
		timeout     = flag.Duration("timeout", 30*time.Second, "Request timeout")
		grammar     = flag.String("grammar", "", "Optional grammar constraint")
		verbose     = flag.Bool("v", false, "Verbose output")
		interactive = flag.Bool("i", false, "Interactive mode (continuous chat)")
	)
	flag.Parse()

	// Show usage if no input and not interactive mode
	if *input == "" && !*interactive {
		fmt.Println("NATS Chat Client - Chat with AI models over NATS messaging")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  # One-shot mode")
		fmt.Println("  nats-chat -subject <subject> -input 'Your message'")
		fmt.Println()
		fmt.Println("  # Interactive mode")  
		fmt.Println("  nats-chat -subject <subject> -i")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # One-shot with default subject")
		fmt.Println("  ./nats-chat -input 'Tell me a joke'")
		fmt.Println()
		fmt.Println("  # Interactive with specific model")
		fmt.Println("  ./nats-chat -subject inference.request.qwen3-4b -i")
		fmt.Println()
		fmt.Println("  # One-shot with custom parameters")
		fmt.Println("  ./nats-chat -subject inference.request.gemma3-270m -input 'Explain quantum physics' -max-tokens 200")
		fmt.Println()
		fmt.Println("  # With grammar constraint") 
		fmt.Println("  ./nats-chat -input 'Generate JSON' -grammar 'json' -temperature 0.1")
		os.Exit(1)
	}

	// Connect to NATS
	nc, err := nats.Connect(*natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	if *verbose {
		fmt.Printf("Connected to NATS: %s\n", *natsURL)
		fmt.Printf("Publishing to subject: %s\n", *subject)
	}

	if *interactive {
		runInteractiveMode(nc, *subject, *maxTokens, *temperature, *grammar, *timeout, *verbose)
	} else {
		runOneShotMode(nc, *subject, *input, *maxTokens, *temperature, *grammar, *timeout, *verbose)
	}
}

func runOneShotMode(nc *nats.Conn, subject, input string, maxTokens int, temperature float64, grammar string, timeout time.Duration, verbose bool) {
	response := sendInferenceRequest(nc, subject, input, maxTokens, temperature, grammar, timeout, verbose)
	
	if response.Error != "" {
		fmt.Printf("❌ Error: %s\n", response.Error)
		os.Exit(1)
	}

	// Print the AI response
	fmt.Print(response.Text)
	
	if verbose {
		fmt.Printf("\n--- Performance: %dms, %d→%d tokens ---\n", 
			response.DurationMs, response.TokensIn, response.TokensOut)
	}
}

func runInteractiveMode(nc *nats.Conn, subject string, maxTokens int, temperature float64, grammar string, timeout time.Duration, verbose bool) {
	fmt.Printf("🤖 Interactive NATS Chat (subject: %s)\n", subject)
	fmt.Println("Type 'quit' or 'exit' to end the session")
	fmt.Println("Type '/help' for commands")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		// Handle commands
		switch input {
		case "quit", "exit":
			fmt.Println("👋 Goodbye!")
			return
		case "/help":
			fmt.Println("Commands:")
			fmt.Println("  quit, exit - End the session")
			fmt.Println("  /help      - Show this help")
			fmt.Println("  /subject <new-subject> - Change NATS subject")
			fmt.Printf("Current subject: %s\n", subject)
			continue
		}
		
		if strings.HasPrefix(input, "/subject ") {
			newSubject := strings.TrimSpace(strings.TrimPrefix(input, "/subject "))
			if newSubject != "" {
				subject = newSubject
				fmt.Printf("✅ Subject changed to: %s\n", subject)
			} else {
				fmt.Println("❌ Invalid subject. Usage: /subject <subject-name>")
			}
			continue
		}
		
		fmt.Println()  // Add newline after user input
		response := sendInferenceRequest(nc, subject, input, maxTokens, temperature, grammar, timeout, verbose)
		
		if response.Error != "" {
			fmt.Printf("❌ Error: %s\n", response.Error)
			continue
		}
		
		if response.Text == "" {
			fmt.Printf("AI: [No response generated - try rephrasing your request]\n")
		} else {
			fmt.Printf("AI: %s", response.Text)
		}
		
		if verbose {
			fmt.Printf(" [%dms, %d→%d tokens]", response.DurationMs, response.TokensIn, response.TokensOut)
		}
		fmt.Println()
	}
}

func sendInferenceRequest(nc *nats.Conn, subject, input string, maxTokens int, temperature float64, grammar string, timeout time.Duration, verbose bool) InferenceResponse {
	// Create temporary reply subject
	replySubject := fmt.Sprintf("reply.%d", time.Now().UnixNano())
	
	// Prepare request with unique ID
	reqID := fmt.Sprintf("cli-%d-%d", time.Now().UnixNano(), os.Getpid())
	
	params := map[string]interface{}{
		"temperature": temperature,
	}
	
	// Only include max_tokens if specified (not -1)
	if maxTokens > 0 {
		params["max_tokens"] = maxTokens
	}
	
	if grammar != "" {
		params["grammar"] = grammar
	}

	request := InferenceRequest{
		ReqID:   reqID,
		Input:   input,
		Params:  params,
		ReplyTo: replySubject, // Include reply topic in message payload
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return InferenceResponse{Error: fmt.Sprintf("Failed to marshal request: %v", err)}
	}

	if verbose {
		fmt.Printf("Request: %s\n", string(requestBytes))
		fmt.Printf("Reply subject: %s\n", replySubject)
	}

	// Subscribe to reply subject
	replyChan := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		return InferenceResponse{Error: fmt.Sprintf("Failed to subscribe to reply: %v", err)}
	}
	defer sub.Unsubscribe()

	// Publish request to JetStream
	if err := nc.Publish(subject, requestBytes); err != nil {
		return InferenceResponse{Error: fmt.Sprintf("Failed to publish request: %v", err)}
	}

	// Start spinner animation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	spinnerWg := startSpinner(ctx, "Waiting for AI response...")

	// Wait for response with timeout
	select {
	case msg := <-replyChan:
		// Stop spinner
		cancel()
		spinnerWg.Wait()
		
		// Parse response
		var response InferenceResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			return InferenceResponse{Error: fmt.Sprintf("Failed to parse response: %v", err)}
		}

		if verbose && response.Error == "" {
			fmt.Printf("Request ID: %s\n", response.ReqID)
			fmt.Printf("Duration: %dms\n", response.DurationMs)
			fmt.Printf("Tokens In: %d\n", response.TokensIn)
			fmt.Printf("Tokens Out: %d\n", response.TokensOut)
			fmt.Println("Response:")
		}

		return response
		
	case <-time.After(timeout):
		// Stop spinner
		cancel()
		spinnerWg.Wait()
		
		return InferenceResponse{Error: fmt.Sprintf("Request timeout after %v", timeout)}
	}
}