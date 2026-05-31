# MCP Workshop Lab

This repository is the hands-on lab environment for the **[AI + Identity Workshop at Identiverse 2026](https://identiverse.com/idv26/ai-identity-workshop/)**. Lab instances are pre-configured EC2 machines accessed via AWS CloudShell — no SSH key, no browser required. All interaction with the AI agent happens through a CLI chat interface.

## What the Lab Covers

The lab walks through MCP security progressively, with each phase swapping only the MCP client configuration:

1. **WORKING** — Credentials hardcoded in the config. The tools work, but secrets are exposed.
2. **SECRETWRAPPED** — Credentials fetched at runtime from AWS Secrets Manager. No secrets in the config file.
3. **XAAIDJAG** — Adds an XAA-protected MCP resource. The agent obtains an ID token, exchanges it for a cross-app authorization grant (ID-JAG), and presents a resource access token — all transparently.

## Lab Environment

The lab runs inside a Docker Compose environment on your assigned EC2 instance. Services:

| Service | Purpose |
|---|---|
| `postgres` | Database with sample movie data |
| `ollama` | Locally hosted LLM |
| `client` | Node.js container — this is where you run the agent |
| `auth-server` | Demo OIDC / enterprise IdP (required for Lab 3) |
| `resource-server` | XAA-protected MCP server — todo list (required for Lab 3) |
| `requesting-app` | XAA-aware requesting app + client provisioning (required for Lab 3) |

## Getting Started

### Connect to Your Lab Instance

From AWS CloudShell:

```bash
aws ssm start-session --target <your_instance_id> \
  --document-name AWS-StartInteractiveCommand \
  --parameters command="cd ~ && exec bash -l"
```

### Start the Lab

```bash
source ~/lab-config.env
cd $WORKSHOP_REPO_DIR
./start-lab.sh --build -d
```

`start-lab.sh` reads `~/lab-config.env`, substitutes the configured model name and repo references into local files, then starts all Docker Compose services. The Ollama model pulls automatically on first run — this may take a few minutes.

### Copy the Binary and Enter the Container

Run these on the lab instance (not inside the container):

```bash
aws s3 cp s3://mcp-lab-instance-setup/xaa-mcp-stdio-linux-amd64 ./bin/xaa-mcp-stdio-linux-amd64
docker cp ./bin/xaa-mcp-stdio-linux-amd64 $CLIENT_CONTAINER:/root/
docker exec -it $CLIENT_CONTAINER bash
```

The binary is the XAA MCP bridge used in Lab 3. Copying it now so it is ready when you need it.

## Lab Walkthrough

All commands below run **inside the client container** unless noted otherwise.

### Set Up the Agent (one-time)

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

### Lab 1 — WORKING (hardcoded secrets)

```bash
npm start
```

You are now in the agent chat UI talking to the locally hosted LLM. Try:

```
you're going to use the "query" tool to get info about movies from the movies table
in the database. when you call the "query" tool be sure you label the SQL as "sql"
in the arguments so that it works correctly
```

Exit with `Ctrl+C` when done.

### Lab 2 — SECRETWRAPPED (secrets from AWS)

```bash
cp ../$WORKSHOP_REPO_DIR/docker-compose-lab-mcp-config-files/SECRETWRAPPED.mcp-config.json mcp-config.json
npm start
```

The same query works — but credentials are no longer in the config file. The MCP secret wrapper fetches them from AWS Secrets Manager at runtime.

Exit with `Ctrl+C` when done.

### Lab 3 — XAAIDJAG (XAA-protected resource)

First provision an OAuth client from the requesting app:

```bash
curl -s -X POST http://requesting-app:3000/api/clients/provision \
  -H "Content-Type: application/json" \
  -d '{"name": "Deadpool"}' | jq .
```

Save the returned `client_id` and `client_secret`. Then:

```bash
cp ../$WORKSHOP_REPO_DIR/docker-compose-lab-mcp-config-files/XAAIDJAG.mcp-config.json mcp-config.json
# edit client_id and client_secret into mcp-config.json
npm start
```

The agent now handles the full XAA token flow automatically — ID token → ID-JAG → resource access token — before reaching the protected MCP server.

## Reference and Troubleshooting

### Useful Commands

```bash
docker compose logs <service>            # logs for a specific service
docker compose logs ollama-model-puller  # check model pull progress
docker ps                                # list running containers
docker exec -it $CLIENT_CONTAINER bash   # re-enter the client container
```

### If the Ollama Model Pull Fails

The model pulls automatically when `./start-lab.sh --build -d` runs. Check progress or errors with:

```bash
docker compose logs ollama-model-puller
```

If the pull failed and the container exited, re-run it:

```bash
docker compose up ollama-model-puller
```

### About the Lab Environment

- The auth server is a demo — it simulates an enterprise IdP but is not a real IdP product.
- Demo users are stored by email in local JSON files; no external user store is required.
- There is no OAuth Dynamic Client Registration — clients are provisioned via the `/api/clients/provision` endpoint.

### Running Outside the Workshop

To run this environment locally, use the browser UI, connect from Cursor or Codex, or step through the XAA token flow manually with curl — see [SUPPLEMENTARY.md](SUPPLEMENTARY.md).
