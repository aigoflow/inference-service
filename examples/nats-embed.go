package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

type EmbeddingRequest struct {
	TraceID string      `json:"trace_id,omitempty"`
	ReqID   string      `json:"req_id"`
	Input   interface{} `json:"input"`    // Can be string or []string
	Model   string      `json:"model,omitempty"`
	ReplyTo string      `json:"reply_to,omitempty"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
	Error  string          `json:"error,omitempty"`
}

func main() {
	var (
		natsURL = flag.String("url", "nats://127.0.0.1:5700", "NATS server URL")
		subject = flag.String("subject", "embedding.request.nomic-embed-v1.5", "NATS subject to publish to")
		input   = flag.String("input", "", "Text input for embedding")
		model   = flag.String("model", "nomic-embed-v1.5", "Model name")
		timeout = flag.Duration("timeout", 10*time.Second, "Request timeout")
		verbose = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	if *input == "" {
		fmt.Println("NATS Embedding Client - Generate embeddings over NATS messaging")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  nats-embed -input 'Your text here'")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./nats-embed -input 'search_query: machine learning'")
		fmt.Println("  ./nats-embed -subject embedding.request.nomic-embed-v2-moe -input 'Hola mundo'")
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

	// Create temporary reply subject
	replySubject := fmt.Sprintf("embed.reply.%d", time.Now().UnixNano())

	// Prepare request with unique ID
	reqID := fmt.Sprintf("embed-cli-%d-%d", time.Now().UnixNano(), os.Getpid())

	request := EmbeddingRequest{
		ReqID:   reqID,
		Input:   *input,
		Model:   *model,
		ReplyTo: replySubject,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	if *verbose {
		fmt.Printf("Request: %s\n", string(requestBytes))
		fmt.Printf("Reply subject: %s\n", replySubject)
	}

	// Subscribe to reply subject
	replyChan := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe(replySubject, func(msg *nats.Msg) {
		replyChan <- msg
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to reply: %v", err)
	}
	defer sub.Unsubscribe()

	// Publish request
	if err := nc.Publish(*subject, requestBytes); err != nil {
		log.Fatalf("Failed to publish request: %v", err)
	}

	fmt.Printf("🔍 Generating embedding for: %s\n", *input)

	// Wait for response with timeout
	select {
	case msg := <-replyChan:
		// Parse response
		var response EmbeddingResponse
		if err := json.Unmarshal(msg.Data, &response); err != nil {
			log.Fatalf("Failed to parse response: %v", err)
		}

		if response.Error != "" {
			fmt.Printf("❌ Error: %s\n", response.Error)
			os.Exit(1)
		}

		if len(response.Data) == 0 {
			fmt.Printf("❌ No embeddings returned\n")
			os.Exit(1)
		}

		// Print embedding info
		embData := response.Data[0]
		fmt.Printf("✅ Embedding generated successfully!\n")
		fmt.Printf("📏 Dimensions: %d\n", len(embData.Embedding))
		fmt.Printf("🔢 Tokens: %d\n", response.Usage.PromptTokens)

		if *verbose {
			fmt.Printf("\n📊 First 10 dimensions: ")
			for i := 0; i < 10 && i < len(embData.Embedding); i++ {
				fmt.Printf("%.6f ", embData.Embedding[i])
			}
			fmt.Println()
		}

	case <-time.After(*timeout):
		log.Fatalf("Request timeout after %v", *timeout)
	}
}