# XAA MCP Demo

This repository is a lightweight, Dockerized demo of Cross App Access (XAA) for MCP.

It includes:

- a demo OIDC / enterprise IdP service
- a protected MCP resource server in front of a todo list
- a requesting app with a browser UI
- a host-facing MCP bridge that real MCP hosts such as Cursor or Codex can connect to

The browser UI shows the full sequence:

1. demo user enrollment by email
2. demo client selection or creation
3. OIDC authorization code + PKCE
4. ID token -> ID-JAG token exchange
5. ID-JAG -> resource access token exchange
6. protected MCP tool and resource access

The host-facing bridge is the practical integration point for real MCP hosts. Cursor or Codex connects to the requesting app's remote MCP endpoint, and the bridge performs the upstream XAA flow before calling the protected MCP resource server.

## Services

- `auth-server`
  - demo OIDC authorization server
  - static demo-client registry
  - enrolled users by email
  - token exchange endpoint that issues ID-JAG JWTs

- `resource-server`
  - protected MCP server
  - OAuth protected resource metadata
  - JWT bearer grant endpoint that accepts ID-JAG assertions
  - per-user todo storage

- `requesting-app`
  - browser UI
  - XAA-aware MCP client
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

- Go
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

1. Open [http://localhost:3000](http://localhost:3000).
2. Enter a demo email such as `alice@example.com`.
3. Leave the selected client as `demo-requesting-app`, or create a new demo client.
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

## Use From Cursor

The bridge endpoint is:

```text
http://localhost:3000/host/mcp
```

Example config is in `examples/cursor.mcp.json`.

Copy that example into a local workspace file at `.cursor/mcp.json` if you want Cursor to pick it up automatically for this repo.

Use headers to select the enrolled demo user and client:

```json
{
  "mcpServers": {
    "xaa-demo": {
      "url": "http://localhost:3000/host/mcp",
      "headers": {
        "X-Demo-User": "alice@example.com",
        "X-Demo-Client": "demo-requesting-app"
      }
    }
  }
}
```

After adding the server, ask Cursor to:

- list todos
- add a todo
- toggle a todo
- delete a todo

Each bridge tool call performs the upstream XAA flow before talking to the protected resource server.

## Use From Codex

Example config is in `examples/codex.config.toml`.

```toml
[mcp_servers.xaa_demo]
url = "http://localhost:3000/host/mcp"
http_headers = { "X-Demo-User" = "alice@example.com", "X-Demo-Client" = "demo-requesting-app" }
```

Once configured, Codex can call the same bridge tools as Cursor.

## Verified Flow

This repo was validated with:

- `go test ./...`
- `npm run build --prefix web`
- `docker compose up --build`
- live HTTP checks for:
  - user enrollment
  - browser-triggered XAA flow
  - host-facing MCP `tools/call`

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
