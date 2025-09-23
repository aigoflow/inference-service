#!/bin/bash

# Inference Endpoints Test Script
# Tests HTTP and NATS inference with different models and parameters

set -e

echo "🚀 Inference Endpoints Test Suite"
echo "================================="
echo

# Test HTTP Inference
echo "🌐 HTTP Inference Tests"
echo "======================="

echo "1. Basic HTTP inference (Gemma3-270M)"
curl -s -X POST "http://localhost:5770/v1/completions" \
  -H "Content-Type: application/json" \
  -d '{"input":"What is 2+2?","params":{"max_tokens":50,"temperature":0.3}}' | jq .
echo

echo "2. HTTP inference with custom parameters (Gemma3-270M)"
curl -s -X POST "http://localhost:5770/v1/completions" \
  -H "Content-Type: application/json" \
  -d '{"input":"Write a haiku about coding","params":{"max_tokens":100,"temperature":0.8,"top_p":0.9}}' | jq .
echo

echo "3. HTTP inference unlimited tokens (Gemma3-270M)"
curl -s -X POST "http://localhost:5770/v1/completions" \
  -H "Content-Type: application/json" \
  -d '{"input":"Explain the concept of recursion","params":{"temperature":0.7}}' | jq .
echo

echo "4. HTTP health check"
curl -s "http://localhost:5770/healthz"
echo -e "\n"

echo "5. HTTP logs check"
curl -s "http://localhost:5770/logs?limit=3" | jq .
echo

# Test NATS Inference
echo "📡 NATS Inference Tests"
echo "======================"

echo "6. NATS inference to Gemma3-270M"
./bin/nats-chat -subject inference.request.gemma3-270m -input "What is machine learning?" -max-tokens 150
echo -e "\n"

echo "7. NATS inference to Qwen3-4B (if available)"
./bin/nats-chat -subject inference.request.qwen3-4b -input "Explain databases" -max-tokens 150 -timeout 60s
echo -e "\n"

echo "8. NATS unlimited tokens"
./bin/nats-chat -subject inference.request.gemma3-270m -input "Tell me about the history of computers"
echo -e "\n"

# Test Different Models
echo "🤖 Multi-Model Comparison"
echo "========================="

echo "9. Same question to both models (if qwen3-4b is ready)"
QUESTION="What is the difference between AI and ML?"

echo "Gemma3-270M response:"
./bin/nats-chat -subject inference.request.gemma3-270m -input "$QUESTION" -max-tokens 200
echo -e "\n"

echo "Qwen3-4B response:"
./bin/nats-chat -subject inference.request.qwen3-4b -input "$QUESTION" -max-tokens 200 -timeout 60s
echo -e "\n"

# Parallel Processing Test
echo "⚡ Parallel Processing Test"
echo "=========================="

echo "10. 3 parallel requests to Gemma3-270M"
./bin/nats-chat -subject inference.request.gemma3-270m -input "Count to 5" -max-tokens 50 &
./bin/nats-chat -subject inference.request.gemma3-270m -input "Name 3 colors" -max-tokens 50 &
./bin/nats-chat -subject inference.request.gemma3-270m -input "Say hello" -max-tokens 50 &

echo "Waiting for parallel requests to complete..."
wait
echo

echo "🎯 Performance Comparison"
echo "========================"

echo "11. Speed test - Short responses"
time ./bin/nats-chat -subject inference.request.gemma3-270m -input "Yes or no?" -max-tokens 5
echo

echo "12. Speed test - Medium responses"  
time ./bin/nats-chat -subject inference.request.gemma3-270m -input "Explain AI in one paragraph" -max-tokens 150
echo

echo "✅ All Inference Tests Complete!"
echo "==============================="
echo "Tested: HTTP API, NATS messaging, multiple models, parallel processing"