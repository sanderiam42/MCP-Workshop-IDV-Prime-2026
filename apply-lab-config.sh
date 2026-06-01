#!/bin/bash
# Run once inside the client container immediately after cloning the workshop repo.
# Substitutes lab-specific values (injected via docker-compose) into config files.
set -euo pipefail

: "${OLLAMA_MODEL:?OLLAMA_MODEL not set — was the container started via start-lab.sh?}"
: "${WORKSHOP_REPO:?WORKSHOP_REPO not set}"
: "${WORKSHOP_REPO_DIR:?WORKSHOP_REPO_DIR not set}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Substitute model name in all MCP config files
sed -i "s|__OLLAMA_MODEL__|${OLLAMA_MODEL}|g" \
  "$SCRIPT_DIR"/docker-compose-lab-mcp-config-files/WORKING.mcp-config.json \
  "$SCRIPT_DIR"/docker-compose-lab-mcp-config-files/SECRETWRAPPED.mcp-config.json \
  "$SCRIPT_DIR"/docker-compose-lab-mcp-config-files/XAAIDJAG.mcp-config.json \
  "$SCRIPT_DIR"/cheatsheet/BROKEN.mcp-config.json \
  "$SCRIPT_DIR"/cheatsheet/SUPPLIEDSECRET.mcp-config.json

# Substitute repo placeholders in the cheatsheet so it's usable as a reference inside the container.
# __CLIENT_CONTAINER__ is intentionally left — it refers to a host-side value not needed inside the container.
sed -i \
  -e "s|__WORKSHOP_REPO_DIR__|${WORKSHOP_REPO_DIR}|g" \
  -e "s|__WORKSHOP_REPO__|${WORKSHOP_REPO}|g" \
  "$SCRIPT_DIR"/cheatsheet/copyPasta.txt

echo "Lab config applied:"
echo "  OLLAMA_MODEL=${OLLAMA_MODEL}"
echo "  WORKSHOP_REPO_DIR=${WORKSHOP_REPO_DIR}"
