# XAA MCP Demo

This repository is the hands-on lab environment for the **[AI + Identity Workshop at Identiverse 2026](https://identiverse.com/idv26/ai-identity-workshop/)**. This document walks through the lab as it is run during the workshop — on a cloud-hosted EC2 instance, accessed via CLI, with no browser required.

The Docker Compose environment here supports more than the workshop lab path: it includes a browser UI, a host-facing MCP bridge for Cursor and Codex, and a full XAA token flow you can exercise directly with curl. Those capabilities are documented in [SUPPLEMENTARY.md](SUPPLEMENTARY.md) for anyone running the environment on their own outside the workshop context.

## What This Lab Covers

The lab walks through MCP security progressively — each phase introduces a new concept by swapping only the MCP client configuration:

1. **WORKING** — MCP tools connect to a live PostgreSQL database with credentials hardcoded in the config. It works, but the secrets are exposed.
2. **SECRETWRAPPED** — same tools, but credentials are fetched at runtime from AWS Secrets Manager via an MCP secret wrapper. No secrets in the config file.
3. **XAAIDJAG** — adds an XAA-protected MCP resource server. The agent must obtain an ID token, exchange it for an ID-JAG (cross-app authorization grant), and present a resource access token to reach the protected server.

## Lab Environment

The lab runs inside a Docker Compose environment on a cloud-hosted EC2 instance. You access it via AWS CloudShell using SSM — no SSH key required. All interaction with the AI agent happens through a CLI chat UI; there is no browser component for the core lab exercises.

Services running inside Docker Compose:

| Service | Purpose |
|---|---|
| `postgres` | Database with sample movie data |
| `ollama` | Locally hosted LLM (model configured via `~/lab-config.env`) |
| `client` | Node.js container you work inside |
| `auth-server` | Demo OIDC / enterprise IdP |
| `resource-server` | XAA-protected MCP server (todo list) |
| `requesting-app` | XAA-aware requesting app + client provisioning API |

## Lab Walkthrough

### Phase 0 — Connect to Your Lab Machine

From AWS CloudShell:

```bash
aws ssm start-session --target <your_instance_id> \
  --document-name AWS-StartInteractiveCommand \
  --parameters command="cd ~ && exec bash -l"
```

### Phase 1 — Start the Services

```bash
cd $WORKSHOP_REPO_DIR
./start-lab.sh --build -d
```

`start-lab.sh` sources `~/lab-config.env`, substitutes the configured model name and repo references into local files, then runs Docker Compose.

Copy the MCP bridge binary from S3 into the client container:

```bash
aws s3 cp s3://mcp-lab-instance-setup/xaa-mcp-stdio-linux-amd64 ./bin/xaa-mcp-stdio-linux-amd64
docker cp ./bin/xaa-mcp-stdio-linux-amd64 $CLIENT_CONTAINER:/root/
```

Open a shell inside the client container:

```bash
docker exec -it $CLIENT_CONTAINER bash
```

### Phase 2 — Set Up the Agent (inside the container)

```bash
cd ~
git clone https://github.com/ausboss/mcp-ollama-agent
git clone $WORKSHOP_REPO
chmod +x xaa-mcp-stdio-linux-amd64
cd mcp-ollama-agent/
cp mcp-config.json ORIG.mcp-config.json
cp ../$WORKSHOP_REPO_DIR/docker-compose-lab-mcp-config-files/WORKING.mcp-config.json mcp-config.json
npm install
```

### Phase 3 — Lab 1: WORKING (hardcoded secrets)

```bash
npm start
```

You are now in the agent chat UI talking to the locally hosted LLM. Try:

```
you're going to use the "query" tool to get info about movies from the movies table
in the database. when you call the "query" tool be sure you label the SQL as "sql"
in the arguments so that it works correctly
```

Exit the chat UI when done (`Ctrl+C`).

### Phase 4 — Lab 2: SECRETWRAPPED (secrets from AWS)

```bash
cp ../$WORKSHOP_REPO_DIR/docker-compose-lab-mcp-config-files/SECRETWRAPPED.mcp-config.json mcp-config.json
npm start
```

The same database query works — but the credentials are no longer in the config file. The MCP secret wrapper fetches them from AWS Secrets Manager at runtime.

Exit the chat UI when done (`Ctrl+C`).

### Phase 5 — Lab 3: XAAIDJAG (XAA-protected resource)

First, provision an OAuth client from the requesting app:

```bash
curl -s -X POST http://requesting-app:3000/api/clients/provision \
  -H "Content-Type: application/json" \
  -d '{"name": "Deadpool"}' | jq .
```

Save the returned `client_id` and `client_secret`. Then copy the XAA config and edit those values in:

```bash
cp ../$WORKSHOP_REPO_DIR/docker-compose-lab-mcp-config-files/XAAIDJAG.mcp-config.json mcp-config.json
# edit client_id and client_secret into mcp-config.json
npm start
```

The agent now obtains an ID token, exchanges it for an ID-JAG, and uses a resource access token to reach the protected MCP server — all transparently through the configured bridge.

## Services Reference

- `auth-server` — demo OIDC authorization server; issues ID tokens and ID-JAG JWTs via token exchange; supports client credentials grant
- `resource-server` — protected MCP server; accepts ID-JAG assertions via JWT bearer grant; per-user todo storage
- `requesting-app` — XAA-aware MCP client; client provisioning API; host-facing remote MCP bridge for Cursor/Codex

## Notes and Simplifications

- The auth server simulates an enterprise IdP — it is not a real IdP product.
- Demo users are stored by email in local JSON files.
- There is no OAuth Dynamic Client Registration.
- The MCP implementation covers `initialize`, `notifications/initialized`, `tools/list`, `tools/call`, `resources/list`, and `resources/read`.

## Going Further

For running the demo locally outside the workshop environment, using it from a browser, connecting Cursor or Codex, or stepping through the XAA token flow manually with curl, see [SUPPLEMENTARY.md](SUPPLEMENTARY.md).
