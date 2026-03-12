#!/bin/bash
# XAA MCP Demo — explicit token-passing, CC-bootstrapped
# Prerequisites: docker compose up --build, jq
#
# Before running this script, provision a client:
#
#   curl -s -X POST http://localhost:3000/api/clients/provision \
#     -H "Content-Type: application/json" \
#     -d '{"name": "my-app"}' | jq .
#
# Copy the returned client_id and client_secret into the variables below.
# The secret is shown only once — save it before continuing.

AUTH_URL="http://localhost:8081"
RESOURCE_URL="http://localhost:8082"

# Paste your provisioned credentials here:
CLIENT_ID="<your-client-id>"
CLIENT_SECRET="<your-client-secret>"

BASIC=$(echo -n "${CLIENT_ID}:${CLIENT_SECRET}" | base64)

echo "=== Step 1: Client credentials grant → ID token ==="
ID_TOKEN=$(curl -s -X POST "${AUTH_URL}/token" \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "scope=mcp:read mcp:write" | jq -r '.id_token')
echo "ID token: ${ID_TOKEN:0:60}..."

echo "=== Step 2: Token exchange — ID token → ID-JAG ==="
ID_JAG=$(curl -s -X POST "${AUTH_URL}/token" \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "subject_token=${ID_TOKEN}" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:id_token" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:id-jag" \
  -d "audience=${RESOURCE_URL}" \
  -d "resource=${RESOURCE_URL}/mcp" \
  -d "scope=mcp:read mcp:write" | jq -r '.access_token')
echo "ID-JAG: ${ID_JAG:0:60}..."

echo "=== Step 3: JWT bearer — ID-JAG → access token ==="
ACCESS_TOKEN=$(curl -s -X POST "${RESOURCE_URL}/oauth/token" \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer" \
  -d "assertion=${ID_JAG}" \
  -d "scope=mcp:read mcp:write" | jq -r '.access_token')
echo "Access token: ${ACCESS_TOKEN:0:60}..."

echo "=== Step 4: MCP initialize ==="
curl -s -X POST "${RESOURCE_URL}/mcp" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"xaa-demo-sh","version":"1.0.0"}}}' | jq .

echo "=== Step 5: List todos ==="
curl -s -X POST "${RESOURCE_URL}/mcp" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_todos","arguments":{}}}' | jq .

echo "=== Step 6: Add a todo ==="
curl -s -X POST "${RESOURCE_URL}/mcp" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"add_todo","arguments":{"text":"hello from xaa-demo.sh"}}}' | jq .
