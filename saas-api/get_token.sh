#!/bin/bash
# Quick script to login and get token

RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@yourapp.com",
    "password": "superadmin123"
  }')

# Try Python first (most reliable)
TOKEN=$(echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token', ''))" 2>/dev/null)

# Fallback to jq if Python fails
if [ -z "$TOKEN" ] || [ "$TOKEN" = "None" ]; then
  TOKEN=$(echo "$RESPONSE" | jq -r '.access_token' 2>/dev/null)
fi

if [ -z "$TOKEN" ] || [ "$TOKEN" = "None" ]; then
  echo "ERROR: Failed to extract token"
  echo "Response: $RESPONSE"
  exit 1
fi

echo "$TOKEN"
