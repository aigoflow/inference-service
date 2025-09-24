package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aigoflow/inference-service/pkg/client"
)

func main() {
	var (
		natsURL     = flag.String("nats", "nats://127.0.0.1:5700", "NATS server URL")
		model       = flag.String("model", "gemma3-270m", "Model to use (gemma3-270m, qwen3-4b, gpt-oss-20b)")
		input       = flag.String("input", "What is 2 + 2?", "Text input for inference")
		temperature = flag.Float64("temperature", 0.7, "Temperature for generation")
		maxTokens   = flag.Int("max-tokens", 50, "Maximum tokens to generate")
		raw         = flag.Bool("raw", false, "Use raw mode (bypass formatting)")
		verbose     = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	fmt.Printf("🚀 Demo Client - Using inference service client package\n")
	fmt.Printf("Model: %s, NATS: %s\n", *model, *natsURL)
	fmt.Printf("Input: %s\n\n", *input)

	// Create inference client using our pkg/client
	inferenceClient, err := client.NewNATSClient(*natsURL, "demo-client")
	if err != nil {
		log.Fatalf("Failed to create inference client: %v", err)
	}
	defer inferenceClient.Close()

	ctx := context.Background()

	// Test 1: Health check
	fmt.Printf("📊 Testing health check...\n")
	health, err := inferenceClient.CheckHealth(ctx, *model)
	if err != nil {
		log.Printf("Health check failed: %v", err)
	} else {
		fmt.Printf("✅ Model %s is %s\n", health.ModelName, health.Status)
		fmt.Printf("   Last activity: %v\n", health.LastActivity.Format("15:04:05"))
		fmt.Printf("   Capabilities: %v\n", health.Capabilities)
		if *verbose {
			fmt.Printf("   Endpoint: %s\n", health.Endpoint)
			fmt.Printf("   NATS Topic: %s\n", health.NATSTopic)
			fmt.Printf("   Version: %s\n", health.Version)
		}
		fmt.Println()
	}

	// Test 2: List available models
	fmt.Printf("📋 Listing available models...\n")
	models, err := inferenceClient.ListModels(ctx)
	if err != nil {
		log.Printf("Failed to list models: %v", err)
	} else {
		fmt.Printf("✅ Available models: %v\n\n", models)
	}

	// Test 3: Text inference
	fmt.Printf("🤖 Testing text inference (%s mode)...\n", map[bool]string{true: "raw", false: "formatted"}[*raw])
	
	params := map[string]interface{}{
		"temperature": *temperature,
		"max_tokens":  *maxTokens,
	}

	start := time.Now()
	var response *client.InferenceResponse
	
	if *raw {
		response, err = inferenceClient.InferRaw(ctx, *model, *input, params)
	} else {
		response, err = inferenceClient.Infer(ctx, *model, *input, params)
	}
	
	if err != nil {
		log.Fatalf("Inference failed: %v", err)
	}

	duration := time.Since(start)
	
	if response.Error != "" {
		fmt.Printf("❌ Inference error: %s\n", response.Error)
		return
	}

	// Display results
	fmt.Printf("✅ Inference successful!\n")
	fmt.Printf("📝 Response: %s\n", response.Text)
	fmt.Printf("📊 Stats:\n")
	fmt.Printf("   - Request ID: %s\n", response.ReqID)
	fmt.Printf("   - Tokens in: %d, out: %d\n", response.TokensIn, response.TokensOut)
	fmt.Printf("   - Server duration: %dms\n", response.DurationMs)
	fmt.Printf("   - Client duration: %v\n", duration)
	fmt.Printf("   - Tokens/sec: %.1f\n", float64(response.TokensOut)/float64(response.DurationMs)*1000)
	
	if *verbose {
		fmt.Printf("\n🔍 Full response JSON:\n")
		responseJSON, _ := json.MarshalIndent(response, "", "  ")
		fmt.Printf("%s\n", responseJSON)
	}

	// Test 4: Test embeddings (if supported)
	fmt.Printf("\n🔤 Testing embeddings...\n")
	embedding, err := inferenceClient.Embed(ctx, *model, "Hello world")
	if err != nil {
		fmt.Printf("⚠️  Embeddings not supported or failed: %v\n", err)
	} else if embedding.Error != "" {
		fmt.Printf("⚠️  Embedding error: %s\n", embedding.Error)
	} else {
		fmt.Printf("✅ Embedding generated!\n")
		fmt.Printf("   - Model: %s\n", embedding.Model)
		fmt.Printf("   - Dimensions: %d\n", len(embedding.Data[0].Embedding))
		fmt.Printf("   - Usage: %d prompt tokens, %d total tokens\n", 
			embedding.Usage.PromptTokens, embedding.Usage.TotalTokens)
	}

	fmt.Printf("\n🎉 Demo completed successfully!\n")
}