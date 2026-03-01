#!/usr/bin/env bash
set -euo pipefail

# get-xaa-token.sh — OIDC Auth Code + PKCE flow for xaa.dev (steps 1-2 of XAA chain).
# The agent handles steps 3-5 internally using the id_token written here.
#
# Required env vars (set these before running, or export them):
#   XAA_CLIENT_ID        — your registered client ID at idp.xaa.dev
#   XAA_CLIENT_SECRET    — your registered client secret
#   XAA_REDIRECT_URI     — must match what you registered (e.g. http://localhost:3000/callback)

IDP_TOKEN_URL="https://idp.xaa.dev/token"
IDP_AUTH_URL="https://idp.xaa.dev/authorize"

# --- Validate required env vars ---
: "${XAA_CLIENT_ID:?XAA_CLIENT_ID is not set}"
: "${XAA_CLIENT_SECRET:?XAA_CLIENT_SECRET is not set}"
: "${XAA_REDIRECT_URI:?XAA_REDIRECT_URI is not set}"

# --- Generate PKCE code_verifier (128 random chars, base64url) ---
CODE_VERIFIER=$(openssl rand -base64 96 | tr -d '\n' | tr '+/' '-_' | tr -d '=')

# --- Derive code_challenge (SHA-256 of verifier, base64url encoded) ---
CODE_CHALLENGE=$(printf '%s' "$CODE_VERIFIER" \
  | openssl dgst -sha256 -binary \
  | openssl base64 \
  | tr '+/' '-_' \
  | tr -d '=\n')

# --- Generate random state ---
STATE=$(openssl rand -hex 16)

# --- Build authorization URL ---
AUTH_URL="${IDP_AUTH_URL}?response_type=code&client_id=$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))" "$XAA_CLIENT_ID")&redirect_uri=$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))" "$XAA_REDIRECT_URI")&scope=openid%20email%20profile&state=${STATE}&code_challenge=${CODE_CHALLENGE}&code_challenge_method=S256"

echo ""
echo "============================================================"
echo " Step 1: Open this URL in your browser and log in:"
echo "============================================================"
echo ""
echo "$AUTH_URL"
echo ""
echo "------------------------------------------------------------"
echo " NOTE: After logging in, the browser will redirect to:"
echo "   $XAA_REDIRECT_URI?code=...&state=..."
echo " This redirect WILL FAIL (localhost isn't running) — that's expected."
echo " Copy the value of the 'code' parameter from the URL bar."
echo "------------------------------------------------------------"
echo ""
printf "Paste the authorization code here: "
read -r AUTH_CODE

if [[ -z "$AUTH_CODE" ]]; then
  echo "Error: no authorization code provided." >&2
  exit 1
fi

# --- Exchange auth code for tokens ---
echo ""
echo "Exchanging authorization code for tokens..."

RESPONSE=$(curl -sf -X POST "$IDP_TOKEN_URL" \
  --user "${XAA_CLIENT_ID}:${XAA_CLIENT_SECRET}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=${AUTH_CODE}" \
  --data-urlencode "redirect_uri=${XAA_REDIRECT_URI}" \
  --data-urlencode "code_verifier=${CODE_VERIFIER}")

ID_TOKEN=$(echo "$RESPONSE" | jq -r '.id_token')

if [[ -z "$ID_TOKEN" || "$ID_TOKEN" == "null" ]]; then
  echo "Error: failed to extract id_token from response." >&2
  echo "Response was: $RESPONSE" >&2
  exit 1
fi

# --- Write token to file for agent pickup ---
echo "$ID_TOKEN" > /tmp/.xaa-id-token
echo ""
echo "Success! id_token written to /tmp/.xaa-id-token"
echo ""
echo "You can also export it manually:"
echo "  export XAA_ID_TOKEN=${ID_TOKEN}"
echo ""
