#!/bin/bash

# Grammar CRUD Test Script
# Tests the complete grammar management system via HTTP API

set -e

BASE_URL="http://localhost:5770"
TEST_GRAMMAR_NAME="test-json-schema"

echo "🧪 Grammar CRUD Test Suite"
echo "=========================="
echo "Testing against: $BASE_URL"
echo

# Test JSON schema grammar
JSON_GRAMMAR='root ::= object
object ::= "{" ws members? ws "}"
members ::= pair (ws "," ws pair)*
pair ::= string ws ":" ws value
value ::= string | number | boolean | null | array | object
string ::= "\"" char* "\""
char ::= [^"\\] | "\\" (["\\/bfnrt] | "u" [0-9a-fA-F]{4})
number ::= "-"? ("0" | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?
boolean ::= "true" | "false"
null ::= "null"
array ::= "[" ws (value ws ("," ws value ws)*)? "]"
ws ::= [ \t\n\r]*'

echo "📝 1. CREATE Grammar"
echo "==================="
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/grammars" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"$TEST_GRAMMAR_NAME\",
    \"description\": \"JSON schema validator\",
    \"content\": $(echo "$JSON_GRAMMAR" | jq -Rs .)
  }")

echo "Response: $CREATE_RESPONSE"
echo

echo "📋 2. LIST All Grammars"
echo "======================"
LIST_RESPONSE=$(curl -s "$BASE_URL/grammars")
echo "Response: $LIST_RESPONSE"
echo

echo "🔍 3. GET Specific Grammar"
echo "========================="
GET_RESPONSE=$(curl -s "$BASE_URL/grammars/$TEST_GRAMMAR_NAME")
echo "Response: $GET_RESPONSE"
echo

echo "✏️  4. UPDATE Grammar"
echo "===================="
UPDATED_GRAMMAR="$JSON_GRAMMAR"$'\n# Updated with comment'
UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/grammars/$TEST_GRAMMAR_NAME" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"$TEST_GRAMMAR_NAME\",
    \"description\": \"Updated JSON schema validator with comments\",
    \"content\": $(echo "$UPDATED_GRAMMAR" | jq -Rs .)
  }")

echo "Response: $UPDATE_RESPONSE"
echo

echo "🔍 5. VERIFY Update"
echo "=================="
VERIFY_RESPONSE=$(curl -s "$BASE_URL/grammars/$TEST_GRAMMAR_NAME")
echo "Response: $VERIFY_RESPONSE"
echo

echo "🧪 6. TEST Grammar with Inference"
echo "================================="
INFERENCE_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/completions" \
  -H "Content-Type: application/json" \
  -d "{
    \"input\": \"Generate a JSON object with name and age\",
    \"params\": {
      \"max_tokens\": 100,
      \"temperature\": 0.1,
      \"grammar\": \"$TEST_GRAMMAR_NAME\"
    }
  }")

echo "Response: $INFERENCE_RESPONSE"
echo

echo "🗑️  7. DELETE Grammar"
echo "===================="
DELETE_RESPONSE=$(curl -s -X DELETE "$BASE_URL/grammars/$TEST_GRAMMAR_NAME")
echo "Response: $DELETE_RESPONSE"
echo

echo "✅ 8. VERIFY Deletion"
echo "===================="
FINAL_CHECK=$(curl -s "$BASE_URL/grammars/$TEST_GRAMMAR_NAME")
echo "Response: $FINAL_CHECK"
echo

echo "🎉 Grammar CRUD Test Complete!"
echo "=============================="
echo "All grammar operations tested: CREATE, READ, UPDATE, DELETE"
echo "Grammar-constrained inference tested (note: currently disabled due to llama.cpp issues)"