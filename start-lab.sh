#!/bin/bash
set -euo pipefail

LAB_ENV="${HOME}/lab-config.env"
if [ ! -f "$LAB_ENV" ]; then
  echo "ERROR: ${LAB_ENV} not found. Run prepareInstanceLabs.sh first." >&2
  exit 1
fi

set -a
source "$LAB_ENV"
set +a

# Derive the client container name from the ollama container name
CLIENT_CONTAINER="${OLLAMA_CONTAINER/ollama/client}"

export OLLAMA_MODEL OLLAMA_CONTAINER CLIENT_CONTAINER WORKSHOP_REPO WORKSHOP_REPO_DIR

echo "Lab config loaded:"
echo "  OLLAMA_MODEL=${OLLAMA_MODEL}"
echo "  OLLAMA_CONTAINER=${OLLAMA_CONTAINER}"
echo "  CLIENT_CONTAINER=${CLIENT_CONTAINER}"
echo "  WORKSHOP_REPO_DIR=${WORKSHOP_REPO_DIR}"
echo ""
echo "Applying lab config in place..."

# In-place substitution — must run __WORKSHOP_REPO_DIR__ before __WORKSHOP_REPO__
# to avoid __WORKSHOP_REPO__ partially matching __WORKSHOP_REPO_DIR__
FILES=(
  docker-compose.yml
  docker-compose-lab-mcp-config-files/WORKING.mcp-config.json
  docker-compose-lab-mcp-config-files/SECRETWRAPPED.mcp-config.json
  docker-compose-lab-mcp-config-files/XAAIDJAG.mcp-config.json
  cheatsheet/BROKEN.mcp-config.json
  cheatsheet/SUPPLIEDSECRET.mcp-config.json
  cheatsheet/copyPasta.txt
)

for f in "${FILES[@]}"; do
  sed -i \
    -e "s|__OLLAMA_MODEL__|${OLLAMA_MODEL}|g" \
    -e "s|__OLLAMA_CONTAINER__|${OLLAMA_CONTAINER}|g" \
    -e "s|__CLIENT_CONTAINER__|${CLIENT_CONTAINER}|g" \
    -e "s|__WORKSHOP_REPO_DIR__|${WORKSHOP_REPO_DIR}|g" \
    -e "s|__WORKSHOP_REPO__|${WORKSHOP_REPO}|g" \
    "$f"
done

echo "Done. Starting Docker Compose..."
exec docker compose up "$@"
