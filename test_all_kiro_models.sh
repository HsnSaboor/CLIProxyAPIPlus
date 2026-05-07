#!/bin/bash
API_KEY="sk-2ws2ZbNo19IHKPMHu1WmyqIH5DeYApo6a1O7H2aflvjlh"
URL="http://localhost:8317/v1/chat/completions"

MODELS=(
  "kiro-glm-5"
  "kiro-glm-5-agentic"
  "kiro-minimax-m2-1"
  "kiro-minimax-m2-5"
  "kiro-minimax-m2-5-agentic"
  "kiro-deepseek-3-2"
  "kiro-deepseek-3-2-agentic"
  "kiro-qwen3-coder-next"
  "kiro-qwen3-coder-next-agentic"
  "kiro-claude-sonnet-4-5-agentic"
)

for MODEL in "${MODELS[@]}"; do
  echo "Testing model: $MODEL"
  RESPONSE=$(curl -s -X POST $URL \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"model\": \"$MODEL\",
      \"messages\": [{\"role\": \"user\", \"content\": \"hello\"}],
      \"max_tokens\": 10
    }")
  
  if echo "$RESPONSE" | grep -q '"error"'; then
    echo "  [ERROR] Response: $RESPONSE"
  elif echo "$RESPONSE" | grep -q '"Improperly formed request"'; then
    echo "  [400 ERROR] Response: $RESPONSE"
  elif echo "$RESPONSE" | grep -q 'Invalid model'; then
    echo "  [INVALID MODEL] Response: $RESPONSE"
  else
    echo "  [SUCCESS] Content: $(echo "$RESPONSE" | jq -r '.choices[0].message.content')"
  fi
  sleep 1
done
