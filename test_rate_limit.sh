#!/bin/bash

echo "Testing rate limit..."
echo ""

for i in {1..15}; do
  echo "Request $i:"
  response=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST http://localhost:8080/v1/sessions \
    -H "Content-Type: application/json" \
    -H "X-Project-ID: proj-rate-test" \
    -d '{"projectId":"proj-rate-test","timeout":300}')
  
  http_code=$(echo "$response" | grep "HTTP_CODE" | cut -d: -f2)
  
  if [ "$http_code" == "429" ]; then
    echo "  ❌ RATE LIMITED (429)"
  elif [ "$http_code" == "201" ]; then
    echo "  ✅ SUCCESS (201)"
  else
    echo "  ⚠️  Code: $http_code"
  fi
  
  # Small delay
  sleep 0.1
done
