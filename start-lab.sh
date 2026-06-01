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
docker compose up "$@"

# Wait for client container, then copy the XAA MCP bridge binary into it
XAA_BINARY="./bin/xaa-mcp-stdio-linux-amd64"
XAA_S3="s3://mcp-lab-instance-setup/xaa-mcp-stdio-linux-amd64"

echo ""
echo "Waiting for client container to be ready..."
ATTEMPTS=0
MAX_ATTEMPTS=30
while [ $ATTEMPTS -lt $MAX_ATTEMPTS ]; do
  if docker ps --filter "name=${CLIENT_CONTAINER}" --format '{{.Names}}' | grep -q "${CLIENT_CONTAINER}"; then
    break
  fi
  ATTEMPTS=$((ATTEMPTS + 1))
  sleep 2
done

if [ $ATTEMPTS -eq $MAX_ATTEMPTS ]; then
  echo "WARNING: Client container did not appear within 60s. Copy the binary manually:" >&2
  echo "  aws s3 cp ${XAA_S3} ${XAA_BINARY}" >&2
  echo "  docker cp ${XAA_BINARY} ${CLIENT_CONTAINER}:/root/" >&2
else
  if [ ! -f "$XAA_BINARY" ]; then
    echo "Downloading XAA MCP bridge binary from S3..."
    aws s3 cp "$XAA_S3" "$XAA_BINARY"
  fi
  echo "Copying XAA MCP bridge binary into ${CLIENT_CONTAINER}..."
  docker cp "$XAA_BINARY" "${CLIENT_CONTAINER}:/root/"
  echo ""
  echo "Lab is ready. Enter the container with:"
  echo "  docker exec -it ${CLIENT_CONTAINER} bash"
fi
