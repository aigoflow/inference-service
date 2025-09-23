#!/bin/bash

# True Parallel Processing Test
# Tests concurrent requests to multiple models to verify parallel execution

set -e

echo "🚀 True Parallel Processing Test"
echo "==============================="
echo "Testing concurrent requests to both models..."
echo

# Create log file for timing
LOG_FILE="/tmp/parallel_test_$(date +%s).log"

echo "Starting parallel requests at: $(date)" | tee $LOG_FILE
echo

# Start timer
START_TIME=$(date +%s.%N)

echo "🔥 Firing 4 concurrent requests:"
echo "1x Qwen3-4B + 3x Gemma3-270M"
echo

# Fire all requests in parallel (no wait between them)
{
    echo "$(date +%H:%M:%S.%3N) [QWEN] Starting long request..."
    time ./bin/nats-chat -subject inference.request.qwen3-4b -input "Explain machine learning algorithms in detail" -timeout 120s > /tmp/qwen_result.txt 2> /tmp/qwen_time.txt
    echo "$(date +%H:%M:%S.%3N) [QWEN] Completed" 
} &
QWEN_PID=$!

{
    echo "$(date +%H:%M:%S.%3N) [GEMMA1] Starting request..."
    time ./bin/nats-chat -subject inference.request.gemma3-270m -input "What is AI?" > /tmp/gemma1_result.txt 2> /tmp/gemma1_time.txt
    echo "$(date +%H:%M:%S.%3N) [GEMMA1] Completed"
} &
GEMMA1_PID=$!

{
    echo "$(date +%H:%M:%S.%3N) [GEMMA2] Starting request..."
    time ./bin/nats-chat -subject inference.request.gemma3-270m -input "Tell me about space" > /tmp/gemma2_result.txt 2> /tmp/gemma2_time.txt
    echo "$(date +%H:%M:%S.%3N) [GEMMA2] Completed"
} &
GEMMA2_PID=$!

{
    echo "$(date +%H:%M:%S.%3N) [GEMMA3] Starting request..."
    time ./bin/nats-chat -subject inference.request.gemma3-270m -input "Explain quantum computing" > /tmp/gemma3_result.txt 2> /tmp/gemma3_time.txt
    echo "$(date +%H:%M:%S.%3N) [GEMMA3] Completed"
} &
GEMMA3_PID=$!

echo "All requests fired! Waiting for completion..."
echo

# Wait for all to complete
wait $QWEN_PID
wait $GEMMA1_PID  
wait $GEMMA2_PID
wait $GEMMA3_PID

# Calculate total time
END_TIME=$(date +%s.%N)
TOTAL_TIME=$(echo "$END_TIME - $START_TIME" | bc)

echo "⏱️  TIMING ANALYSIS"
echo "=================="
echo "Total wall clock time: ${TOTAL_TIME}s"
echo

echo "📊 Individual Request Times:"
echo "-------------------------"

if [ -f /tmp/qwen_time.txt ]; then
    QWEN_TIME=$(grep "real" /tmp/qwen_time.txt | awk '{print $2}')
    echo "Qwen3-4B:    $QWEN_TIME"
fi

if [ -f /tmp/gemma1_time.txt ]; then
    GEMMA1_TIME=$(grep "real" /tmp/gemma1_time.txt | awk '{print $2}')
    echo "Gemma3-270M #1: $GEMMA1_TIME"
fi

if [ -f /tmp/gemma2_time.txt ]; then
    GEMMA2_TIME=$(grep "real" /tmp/gemma2_time.txt | awk '{print $2}')
    echo "Gemma3-270M #2: $GEMMA2_TIME"
fi

if [ -f /tmp/gemma3_time.txt ]; then
    GEMMA3_TIME=$(grep "real" /tmp/gemma3_time.txt | awk '{print $2}')
    echo "Gemma3-270M #3: $GEMMA3_TIME"
fi

echo
echo "📈 PARALLEL PROCESSING ANALYSIS:"
echo "==============================="

# Calculate sum of individual times if executed sequentially
if [ -f /tmp/qwen_time.txt ] && [ -f /tmp/gemma1_time.txt ] && [ -f /tmp/gemma2_time.txt ] && [ -f /tmp/gemma3_time.txt ]; then
    # Extract seconds from time format (0m2.345s -> 2.345)
    QWEN_SEC=$(echo $QWEN_TIME | sed 's/0m//;s/s//')
    GEMMA1_SEC=$(echo $GEMMA1_TIME | sed 's/0m//;s/s//')
    GEMMA2_SEC=$(echo $GEMMA2_TIME | sed 's/0m//;s/s//')  
    GEMMA3_SEC=$(echo $GEMMA3_TIME | sed 's/0m//;s/s//')
    
    SEQUENTIAL_TIME=$(echo "$QWEN_SEC + $GEMMA1_SEC + $GEMMA2_SEC + $GEMMA3_SEC" | bc)
    SPEEDUP=$(echo "scale=2; $SEQUENTIAL_TIME / $TOTAL_TIME" | bc)
    
    echo "Sequential execution would take: ${SEQUENTIAL_TIME}s"
    echo "Parallel execution took:         ${TOTAL_TIME}s"
    echo "Speedup achieved:               ${SPEEDUP}x"
    echo
    
    if (( $(echo "$SPEEDUP > 2.0" | bc -l) )); then
        echo "✅ EXCELLENT parallel processing! (>2x speedup)"
    elif (( $(echo "$SPEEDUP > 1.5" | bc -l) )); then
        echo "✅ GOOD parallel processing! (>1.5x speedup)"
    else
        echo "⚠️  Limited parallel benefit (<1.5x speedup)"
    fi
else
    echo "⚠️  Could not calculate speedup (missing timing data)"
fi

echo
echo "📝 Response Samples:"
echo "==================="

echo "Qwen3-4B response (first 200 chars):"
if [ -f /tmp/qwen_result.txt ]; then
    head -c 200 /tmp/qwen_result.txt
    echo "..."
else
    echo "❌ No response from Qwen3-4B"
fi
echo

echo "Gemma3-270M #1 response (first 100 chars):"  
if [ -f /tmp/gemma1_result.txt ]; then
    head -c 100 /tmp/gemma1_result.txt
    echo "..."
else
    echo "❌ No response from Gemma3-270M #1"
fi
echo

# Cleanup temp files
rm -f /tmp/{qwen,gemma1,gemma2,gemma3}_{result,time}.txt
rm -f $LOG_FILE

echo "✅ Parallel processing test complete!"
echo "===================================="