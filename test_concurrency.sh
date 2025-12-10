#!/bin/bash

echo "Creating 11 sessions for proj-test..."
echo ""

for i in {1..11}; do
  echo "Creating session $i..."
  response=$(curl -s -X POST http://localhost:8080/v1/sessions \
    -H "Content-Type: application/json" \
    -d '{"projectId":"proj-test","timeout":300}')
  
  if echo "$response" | grep -q "concurrency limit"; then
    echo "❌ Session $i REJECTED: Concurrency limit reached!"
  else
    session_id=$(echo "$response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Session $i created: $session_id"
  fi
  echo ""
done
