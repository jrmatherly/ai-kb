#!/bin/bash

# Test script for the agent configuration feature

set -e

echo "========================================="
echo "Agent configuration feature test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Settings
API_BASE_URL="http://localhost:8080"
KB_ID="kb-00000001"  # Change to your knowledge base ID
TENANT_ID="1"

echo "Settings:"
echo "  API URL: ${API_BASE_URL}"
echo "  Knowledge base ID: ${KB_ID}"
echo "  Workspace ID: ${TENANT_ID}"
echo ""

# Test 1: get the current configuration
echo -e "${YELLOW}Test 1: get the current configuration${NC}"
echo "GET ${API_BASE_URL}/api/v1/initialization/config/${KB_ID}"
RESPONSE=$(curl -s -X GET "${API_BASE_URL}/api/v1/initialization/config/${KB_ID}")
echo "Response:"
echo "$RESPONSE" | jq '.data.agent' || echo "$RESPONSE"
echo ""

# Test 2: save the agent configuration
echo -e "${YELLOW}Test 2: save the agent configuration${NC}"
echo "POST ${API_BASE_URL}/api/v1/initialization/initialize/${KB_ID}"

# Prepare test data (must contain the full configuration)
TEST_DATA='{
  "llm": {
    "source": "local",
    "modelName": "qwen3:0.6b",
    "baseUrl": "",
    "apiKey": ""
  },
  "embedding": {
    "source": "local",
    "modelName": "nomic-embed-text:latest",
    "baseUrl": "",
    "apiKey": "",
    "dimension": 768
  },
  "rerank": {
    "enabled": false
  },
  "multimodal": {
    "enabled": false
  },
  "documentSplitting": {
    "chunkSize": 512,
    "chunkOverlap": 100,
    "separators": ["\n\n", "\n", "\u3002", "\uff01", "\uff1f", ";", "\uff1b"]
  },
  "nodeExtract": {
    "enabled": false
  },
  "agent": {
    "enabled": true,
    "maxIterations": 8,
    "temperature": 0.8,
    "allowedTools": ["knowledge_search", "multi_kb_search", "list_knowledge_bases"]
  }
}'

RESPONSE=$(curl -s -X POST "${API_BASE_URL}/api/v1/initialization/initialize/${KB_ID}" \
  -H "Content-Type: application/json" \
  -d "$TEST_DATA")

if echo "$RESPONSE" | grep -q '"success":true'; then
  echo -e "${GREEN}✓ Agent configuration saved${NC}"
  echo "$RESPONSE" | jq '.' || echo "$RESPONSE"
else
  echo -e "${RED}✗ Failed to save the agent configuration${NC}"
  echo "$RESPONSE"
fi
echo ""

# Wait briefly so the data is saved
sleep 1

# Test 3: verify the configuration was saved
echo -e "${YELLOW}Test 3: verify the configuration was saved${NC}"
echo "GET ${API_BASE_URL}/api/v1/initialization/config/${KB_ID}"
RESPONSE=$(curl -s -X GET "${API_BASE_URL}/api/v1/initialization/config/${KB_ID}")
AGENT_CONFIG=$(echo "$RESPONSE" | jq '.data.agent')

echo "Agent configuration:"
echo "$AGENT_CONFIG" | jq '.'

# Check that the configuration is correct
ENABLED=$(echo "$AGENT_CONFIG" | jq -r '.enabled')
MAX_ITER=$(echo "$AGENT_CONFIG" | jq -r '.maxIterations')
TEMP=$(echo "$AGENT_CONFIG" | jq -r '.temperature')

if [ "$ENABLED" == "true" ] && [ "$MAX_ITER" == "8" ] && [ "$TEMP" == "0.8" ]; then
  echo -e "${GREEN}✓ Configuration verified - all values correct${NC}"
else
  echo -e "${RED}✗ Configuration verification failed${NC}"
  echo "  enabled: $ENABLED (expected: true)"
  echo "  maxIterations: $MAX_ITER (expected: 8)"
  echo "  temperature: $TEMP (expected: 0.8)"
fi
echo ""

# Test 4: get the configuration through the Tenant API
echo -e "${YELLOW}Test 4: get the configuration through the Tenant API${NC}"
echo "GET ${API_BASE_URL}/api/v1/tenants/${TENANT_ID}/agent-config"
RESPONSE=$(curl -s -X GET "${API_BASE_URL}/api/v1/tenants/${TENANT_ID}/agent-config")
echo "Response:"
echo "$RESPONSE" | jq '.' || echo "$RESPONSE"
echo ""

# Test 5: database check (if the database is reachable)
echo -e "${YELLOW}Test 5: database check${NC}"
echo "Tip: run the following SQL query by hand to verify the data:"
echo ""
echo "MySQL:"
echo "  mysql -u root -p weknora -e \"SELECT id, agent_config FROM tenants WHERE id = ${TENANT_ID};\""
echo ""
echo "PostgreSQL:"
echo "  psql -U postgres -d weknora -c \"SELECT id, agent_config FROM tenants WHERE id = ${TENANT_ID};\""
echo ""

echo "========================================="
echo "Tests finished."
echo "========================================="
echo ""
echo "If every test passed, the agent configuration feature works."
echo "If a test failed, check:"
echo "  1. The backend service is running"
echo "  2. The database migrations have run"
echo "  3. The knowledge base ID is correct"
echo ""

