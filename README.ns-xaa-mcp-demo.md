# XAA MCP Demo

This repository is a lightweight, Dockerized demo of Cross App Access (XAA) for MCP.

It includes:

- a demo OIDC / enterprise IdP service
- a protected MCP resource server in front of a todo list
- a requesting app with a browser UI
- a host-facing MCP bridge that real MCP hosts such as Cursor or Codex can connect to
- **authorization code + PKCE flow** (user-delegated): full OIDC login → ID token → ID-JAG → resource access
- **client credentials flow** (machine-to-machine): client authenticates directly → ID-JAG → resource access, no user needed

The browser UI shows the full sequence for both flows.

The host-facing bridge is the practical integration point for real MCP hosts. Cursor or Codex connects to the requesting app's remote MCP endpoint, and the bridge performs the upstream XAA flow before calling the protected MCP resource server.

## Services

- `auth-server`
  - demo OIDC authorization server
  - static demo-client registry
  - enrolled users by email
  - token exchange endpoint that issues ID-JAG JWTs
  - client credentials grant that issues a machine ID token (no user); token exchange then produces the ID-JAG

- `resource-server`
  - protected MCP server
  - OAuth protected resource metadata
  - JWT bearer grant endpoint that accepts ID-JAG assertions
  - per-user todo storage

- `requesting-app`
  - browser UI
  - XAA-aware MCP client (authorization code + PKCE and client credentials)
  - host-facing remote MCP bridge for Cursor/Codex

## Layout

- `cmd/auth-server`
- `cmd/resource-server`
- `cmd/requesting-app`
- `internal/authserver`
- `internal/resourceserver`
- `internal/requestingapp`
- `internal/shared`
- `web`
- `examples`

## Quick Start

### Local prerequisites

- Go 1.25+
- Node.js 20+
- Docker with `docker compose`

### Run tests

```bash
go test ./...
```

### Build the frontend

```bash
npm install --prefix web
npm run build --prefix web
```

The manual frontend build is mainly for local non-Docker runs. The requesting-app container build already compiles the frontend bundle during `docker compose up --build`.

### Start the full demo

```bash
docker compose up --build
```

### Provision a client

After the services are running, create a client to use with the demo (the secret is returned only once):

```bash
curl -s -X POST http://localhost:3000/api/clients/provision \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app"}' | jq .
```

Response:
```json
{
  "client_id": "my-app-a1b2c3d4",
  "client_secret": "...",
  "auth": { ... },
  "resource": { ... }
}
```

Save the `client_secret` — it is not shown again. Use the returned `client_id` and `client_secret` wherever the examples below refer to `<your-client-id>` and `<your-client-secret>`.

Open:

- requesting app UI: [http://localhost:3000](http://localhost:3000)
- auth server: [http://localhost:8081](http://localhost:8081)
- resource server: [http://localhost:8082](http://localhost:8082)

Stop it with:

```bash
docker compose down
```

Reset local demo state with:

```bash
make reset-state
```

## Browser Demo

### Authorization Code + PKCE Flow (User-Delegated)

1. Open [http://localhost:3000](http://localhost:3000).
2. Provision a client (see [Provision a client](#provision-a-client) above) and enter the returned client ID.
3. Enter a demo email such as `alice@example.com`.
4. Click `Enroll User`.
5. Click `Run List Flow` or `Add Todo Through XAA`.
6. Inspect the full trace, including:
   - the initial `401` bearer challenge
   - protected resource metadata
   - OIDC metadata
   - auth code redirect
   - ID token
   - ID-JAG
   - resource access token
   - final MCP requests and responses

### Client Credentials Flow (Machine-to-Machine, Browser)

1. Provision a client first (see [Provision a client](#provision-a-client) above) and save the returned credentials.
2. Open [http://localhost:3000](http://localhost:3000).
3. Leave the user email field blank (no user enrollment needed).
4. Enter the provisioned client ID and client secret.
5. Click `Run Client Credentials Flow`.
6. Inspect the trace — you will see:
   - the initial `401` bearer challenge
   - protected resource metadata
   - OIDC metadata
   - `client_credentials` grant → ID token (machine ID token with sub=client_id)
   - token exchange → ID-JAG
   - resource access token
   - final MCP requests and responses

## Register a Service Client

Provision a client with a generated ID and secret (the secret is returned only once):

```bash
curl -s -X POST http://localhost:3000/api/clients/provision \
  -H "Content-Type: application/json" \
  -d '{"name": "my-service"}' | jq .
```

Response:
```json
{
  "client_id": "my-service-a1b2c3d4",
  "client_secret": "abc123...",
  "auth": { ... },
  "resource": { ... }
}
```

Save the `client_secret` — it is not shown again. Each call to `/api/clients/provision` generates a fresh, unique client ID and secret.

Use those values in your MCP config (see Cursor / Codex sections below).

## Use From Cursor

The bridge endpoint is:

```text
http://localhost:3000/host/mcp
```

Example config is in `examples/cursor.mcp.json`.

Copy that example into a local workspace file at `.cursor/mcp.json` if you want Cursor to pick it up automatically for this repo.

### User-delegated flow (authorization code + PKCE)

```json
{
  "mcpServers": {
    "xaa-demo-user": {
      "url": "http://localhost:3000/host/mcp",
      "headers": {
        "X-Demo-User": "alice@example.com",
        "X-Demo-Client": "<your-client-id>"
      }
    }
  }
}
```

### Machine-to-machine flow (client credentials)

Provision a client first (see [Register a Service Client](#register-a-service-client)), then:

```json
{
  "mcpServers": {
    "xaa-demo-machine": {
      "url": "http://localhost:3000/host/mcp",
      "headers": {
        "X-Demo-Client": "<your-client-id>",
        "X-Demo-Client-Secret": "<your-client-secret>"
      }
    }
  }
}
```

After adding the server, ask Cursor to list todos, add a todo, toggle a todo, or delete a todo. Each bridge tool call performs the upstream XAA flow before talking to the protected resource server.

## Use From Codex

Example config is in `examples/codex.config.toml`.

### User-delegated flow

```toml
[mcp_servers.xaa_demo_user]
url = "http://localhost:3000/host/mcp"
http_headers = { "X-Demo-User" = "alice@example.com", "X-Demo-Client" = "<your-client-id>" }
```

### Machine-to-machine flow

Provision a client first (see [Register a Service Client](#register-a-service-client)), then:

```toml
[mcp_servers.xaa_demo_machine]
url = "http://localhost:3000/host/mcp"
http_headers = { "X-Demo-Client" = "<your-client-id>", "X-Demo-Client-Secret" = "<your-client-secret>" }
```

Once configured, Codex can call the same bridge tools as Cursor.

## Command-Line Demo — Full XAA Flow

`examples/xaa-demo.sh` shows the complete three-step token chain started by client credentials, passing each token explicitly to the next step:

1. **CC grant** → ID token (machine identity token with `sub=client_id`)
2. **Token exchange** → ID-JAG (cross-app authorization grant)
3. **JWT bearer** → resource access token
4. MCP calls using the access token

Provision a client, then run the script:

```bash
docker compose up --build
curl -s -X POST http://localhost:3000/api/clients/provision \
  -H "Content-Type: application/json" \
  -d '{"name": "demo-script"}' | jq .
# Paste the returned client_id and client_secret into examples/xaa-demo.sh
chmod +x examples/xaa-demo.sh && examples/xaa-demo.sh
```

### Step-by-step via curl

Provision a client first, then:

```bash
CLIENT_ID="<your-client-id>"
CLIENT_SECRET="<your-client-secret>"
```

#### Step 1 — CC grant → ID token

```bash
BASIC=$(echo -n "${CLIENT_ID}:${CLIENT_SECRET}" | base64)
ID_TOKEN=$(curl -s -X POST http://localhost:8081/token \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "scope=mcp:read mcp:write" | jq -r '.id_token')
```

#### Step 2 — Token exchange → ID-JAG

```bash
ID_JAG=$(curl -s -X POST http://localhost:8081/token \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
  -d "subject_token=${ID_TOKEN}" \
  -d "subject_token_type=urn:ietf:params:oauth:token-type:id_token" \
  -d "requested_token_type=urn:ietf:params:oauth:token-type:id-jag" \
  -d "audience=http://localhost:8082" \
  -d "resource=http://localhost:8082/mcp" \
  -d "scope=mcp:read mcp:write" | jq -r '.access_token')
```

#### Step 3 — JWT bearer → access token

```bash
ACCESS_TOKEN=$(curl -s -X POST http://localhost:8082/oauth/token \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer" \
  -d "assertion=${ID_JAG}" \
  -d "scope=mcp:read mcp:write" | jq -r '.access_token')
```

#### Step 4 — Call an MCP tool directly

```bash
curl -s -X POST http://localhost:8082/mcp \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_todos","arguments":{}}}' | jq .
```

#### Via the bridge (all three steps performed automatically)

```bash
curl -s -X POST http://localhost:3000/host/mcp \
  -H "Content-Type: application/json" \
  -H "X-Demo-Client: ${CLIENT_ID}" \
  -H "X-Demo-Client-Secret: ${CLIENT_SECRET}" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_todos","arguments":{}}}' | jq .
```

## Verbose Debug Logging

Set `VERBOSE=true` on any service to enable full HTTP and token logging. Set `LOG_FILE` to write to a file (default: stderr only).

Example (docker compose — uncomment in `docker-compose.yml`):
```yaml
# - VERBOSE=true
# - LOG_FILE=/data/debug.log
```

Example (local run):
```bash
VERBOSE=true LOG_FILE=/tmp/xaa-debug.log ./auth-server
```

The log captures:
- Every inbound and outbound HTTP request: method, URL, headers, body
- Every HTTP response: status, headers, body
- Every token issued or received: type, claims, expiry
- Every flow step as it executes

## Verified Flow

This repo was validated with:

- `go test ./...`
- `npm run build --prefix web`
- `docker compose up --build`
- live HTTP checks for:
  - user enrollment
  - browser-triggered XAA flow (authorization code + PKCE)
  - browser-triggered client credentials flow
  - host-facing MCP `tools/call` (both user-delegated and machine-to-machine)

## Notes And Simplifications

- There is no standalone enterprise IdP product here; the auth server simulates that role.
- Demo users are stored by email in local JSON files.
- There is no OAuth Dynamic Client Registration.
- The host-facing bridge is intentionally the real-world integration point for Cursor/Codex.
- The MCP implementation is intentionally small and focuses on:
  - `initialize`
  - `notifications/initialized`
  - `tools/list`
  - `tools/call`
  - `resources/list`
  - `resources/read`

## Local-Only Generated Files

These files are intentionally kept out of Git:

- `data/` state JSON and signing keys
- `.cursor/mcp.json`
- `web/dist/`
- `web/node_modules/`
- `web/*.tsbuildinfo`

## Handy Commands

```bash
make test
make web-build
make up
make down
make reset-state
```
