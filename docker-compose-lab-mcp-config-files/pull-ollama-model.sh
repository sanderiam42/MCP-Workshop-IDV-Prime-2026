#!/bin/sh

show_error() {
  echo ''
  echo '========================================'
  echo '!!!!!!!!!!  ERROR  !!!!!!!!!!'
  echo '========================================'
  echo 'OLLAMA MODEL PULLER FAILED!'
  echo ''
  echo "$1"
  echo ''
  echo 'Check the logs above for details.'
  echo '========================================'
  echo ''
  exit 1
}

MAX_WAIT=150
CHECK_INTERVAL=2
MAX_ATTEMPTS=$((MAX_WAIT / CHECK_INTERVAL))

echo 'Waiting for client container to start...'
ATTEMPTS=0
CLIENT_CONTAINER=''
while [ $ATTEMPTS -lt $MAX_ATTEMPTS ]; do
  # Find client container
  CLIENT_CONTAINER=$(docker ps --filter 'name=client' --format '{{.Names}}' | head -1)
  if [ -n "$CLIENT_CONTAINER" ]; then
    echo "Found client container: $CLIENT_CONTAINER"
    break
  fi
  ATTEMPTS=$((ATTEMPTS + 1))
  sleep $CHECK_INTERVAL
done

if [ -z "$CLIENT_CONTAINER" ]; then
  show_error 'Client container did not start within 2.5 minutes. Please check the client service logs.'
fi

echo 'Waiting for Ollama container to start...'
ATTEMPTS=0
OLLAMA_CONTAINER=''
while [ $ATTEMPTS -lt $MAX_ATTEMPTS ]; do
  # Find ollama container but exclude model-puller containers
  OLLAMA_CONTAINER=$(docker ps --filter 'name=ollama' --format '{{.Names}}' | grep -v 'model-puller' | head -1)
  if [ -n "$OLLAMA_CONTAINER" ]; then
    echo "Found ollama container: $OLLAMA_CONTAINER"
    break
  fi
  ATTEMPTS=$((ATTEMPTS + 1))
  sleep $CHECK_INTERVAL
done

if [ -z "$OLLAMA_CONTAINER" ]; then
  show_error 'Ollama container did not start within 2.5 minutes. Please check the ollama service logs.'
fi

echo 'Waiting for Ollama API to be ready...'
ATTEMPTS=0
while [ $ATTEMPTS -lt $MAX_ATTEMPTS ]; do
  echo "Checking Ollama API readiness (attempt $((ATTEMPTS + 1))/$MAX_ATTEMPTS)..."
  if docker exec "$CLIENT_CONTAINER" sh -c 'curl -s http://ollama:11434/api/tags > /dev/null 2>&1' 2>/dev/null; then
    echo 'Ollama API is ready!'
    break
  fi
  ATTEMPTS=$((ATTEMPTS + 1))
  sleep $CHECK_INTERVAL
done

if [ $ATTEMPTS -eq $MAX_ATTEMPTS ]; then
  show_error 'Ollama API did not become ready within 2.5 minutes. Please check the ollama service logs.'
fi

echo 'Pulling llama3.2:1b model...'
if ! docker exec "$OLLAMA_CONTAINER" ollama pull llama3.2:1b; then
  show_error 'Failed to pull llama3.2:1b model. Check the error messages above for details.'
fi

echo ''
echo '========================================'
echo 'SUCCESS!'
echo '========================================'
echo 'Model llama3.2:1b has been pulled successfully!'
echo '========================================'
echo ''

