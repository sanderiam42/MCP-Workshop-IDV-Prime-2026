# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose

This repository contains lab materials for an MCP (Model Context Protocol) workshop. It is **not a software project with a build system** — it is a collection of configuration files, Docker infrastructure, and instructional assets for running hands-on MCP labs.

> **Security note from README**: Do NOT use any of the Docker images in a production setting. They are intentionally built to demonstrate an insecure, bad-state configuration for educational purposes.

## Lab Environment Architecture

The workshop has two main lab tracks, each with its own MCP configuration files:

### Track 1: Docker Compose Lab
Uses `docker-compose.yml` to spin up three services on a shared `training-network`:
- **postgres** — PostgreSQL 16 with a demo DB (`demo`/`demouser`/`demopass123`), initialized by `pg-init/01_init.sql` (tables: `employees`, `movies`)
- **ollama** — Local LLM server running `llama3.2:1b`, accessible at `http://ollama:11434`
- **client** — `node:20-slim` container with `@modelcontextprotocol/server-filesystem`, `@mzxrai/mcp-webresearch`, and `pg-mcp-server` installed globally

MCP config files for this track live in `docker-compose-lab-mcp-config-files/`.

### Track 2: Claude Desktop + AccuWeather API Lab
Uses Claude Desktop locally with the AccuWeather developer API. MCP config files for this track live in `claude-desktop-lab-mcp-config-files/`.

## Docker Compose Commands

```bash
# Start all services
docker compose start

# Shell into the client container
docker exec -it mcp-workshop-idv-prime-2026-client-1 bash

# View logs for a specific service
docker compose logs ollama
```

## Lab Config File Progression

Each track has config files representing different workshop phases:

| File | Purpose |
|------|---------|
| `WORKING.*` | Working config with hardcoded credentials — used in first lab phase |
| `BROKEN.*` | Intentionally broken credentials — used to demonstrate credential exposure risk |
| `SECRETWRAPPED.*` | Credentials replaced with AWS Secrets Manager ARNs via `mcp-secret-wrapper` — used to demonstrate secrets management |
| `SUPPLIEDSECRET.*` | Like SECRETWRAPPED but with a real pre-supplied ARN filled in |

The `cheatsheet/copyPasta.txt` contains the exact shell commands used during the Docker Compose lab flow (AWS SSM session start, git clones, config file swaps, `npm install`, `npm start`).

## MCP Config Structure

The `EXAMPLE.mcp-client-config.json` documents the three MCP transport types:
- **stdio** — client launches a local subprocess; JSON-RPC over stdin/stdout
- **http** (Streamable HTTP) — remote server, no local process
- **custom** — fully client/server-defined transport

The `mcpServers` key structure and transport type strings (`"stdio"`, `"http"`) are **client conventions**, not mandated by the MCP spec.

## The `mcp-secret-wrapper` Pattern

The `SECRETWRAPPED` configs demonstrate wrapping an MCP server with secrets injection:
```json
{
  "command": "npx",
  "args": [
    "-y",
    "git+https://github.com/astrix-security/mcp-secret-wrapper",
    "ENV_VAR_NAME=<AWS_SECRET_ARN>",
    "--",
    "<actual-mcp-server-command>"
  ],
  "env": {
    "VAULT_TYPE": "aws",
    "VAULT_REGION": "us-east-2"
  }
}
```
The wrapper resolves the secret ARN at runtime and injects the value as an environment variable before launching the wrapped MCP server.
