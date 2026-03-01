# Claude Code Prompt: Add OIDC/XAA Todo0 MCP Integration to MCP Workshop

## Context

You are working on the repository at `https://github.com/sanderiam-astrix/MCP-Workshop-Astrix-Academy-2026` (or its fork at `https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026`). Clone whichever is the working repo.

This is an MCP (Model Context Protocol) workshop lab. It currently uses Docker Compose to run three containers: Ollama (LLM, running `llama3.2:1b`), PostgreSQL (data), and a client container (`mcp-workshop-astrix-academy-2026-client-1`) running a fork of `https://github.com/ausboss/mcp-ollama-agent` — a TypeScript CLI chat agent that connects MCP servers to Ollama.

The workshop currently has two MCP config file variants that students swap between:

- `WORKING.mcp-config.json` — hardcoded Postgres credentials (the insecure starting point)
- `SECRETWRAPPED.mcp-config.json` — credentials fetched via Astrix mcp-secret-wrapper from a vault

These live in `docker-compose-lab-mcp-config-files/` (for Docker Compose labs) and `claude-desktop-lab-mcp-config-files/` (for Claude Desktop labs). There is also a `cheatsheet/` directory with student-facing copy-paste instructions, and a `README.md` at the repo root.

**Your job is to add a third MCP config variant — `OIDCTODO0.mcp-config.json` — that connects the agent to a remote Todo0 MCP server hosted at `xaa.dev` using OIDC/XAA (Cross App Access) authentication.** This requires modifying the mcp-ollama-agent code, creating new files, and updating existing documentation.

Do NOT restructure the repo into "phases" or rename/reorganize existing files. Add the new capability alongside the existing setup, following the patterns already established.

---

## What xaa.dev Is

xaa.dev is Okta's Cross App Access (XAA) testing playground. It hosts:

- **IDP (Identity Provider):** `https://idp.xaa.dev` — handles user login (OIDC Auth Code + PKCE) and token exchange
- **Todo0 Authorization Server:** `https://auth.resource.xaa.dev` — validates ID-JAGs, issues access tokens
- **Todo0 MCP Server:** `https://mcp.xaa.dev/mcp` — StreamableHTTP MCP endpoint, requires Bearer token
- **Todo0 REST API:** `https://api.resource.xaa.dev` — backend API (MCP server proxies to this)
- **Protected Resource Metadata:** `https://mcp.xaa.dev/.well-known/oauth-protected-resource` (RFC 9728)

The Todo0 MCP server exposes **MCP resources** (NOT tools):

- `todo0://todos` — all todos for authenticated user
- `todo0://todos/completed` — completed todos
- `todo0://todos/incomplete` — incomplete todos
- `todo0://todos/stats` — statistics (total, completed, incomplete)

Supported MCP methods: `initialize`, `resources/list`, `resources/read`
Required scope: `todos.read`
Server name: "Todo0 Resource Server" v1.0.0

**Critical:** This server uses MCP resources, not tools. The current mcp-ollama-agent only supports tools (`listTools`/`callTool`). You must extend it to also support resources (`listResources`/`readResource`).

---

## XAA Authentication Flow

The full authentication chain to get a Bearer token for the MCP server:

1. **OIDC Login:** User browser → `https://idp.xaa.dev/authorize?...` (Auth Code + PKCE) → authorization code
2. **Code Exchange:** Authorization code + PKCE verifier + client credentials → POST `https://idp.xaa.dev/token` → ID Token
3. **Token Exchange (RFC 8693):** ID Token + client creds → POST `https://idp.xaa.dev/token` → ID-JAG (5 min TTL, `aud=auth.resource.xaa.dev`)
4. **JWT Bearer Grant (RFC 7523):** ID-JAG + resource client creds → POST `https://auth.resource.xaa.dev/token` → Access Token (2 hr TTL)
5. **MCP Communication:** Access Token as Bearer header → POST `https://mcp.xaa.dev/mcp` → MCP session

Steps 3-5 can be handled by the MCP SDK's `withCrossAppAccess()` middleware if given the ID Token from step 2. Steps 1-2 require browser interaction (handled by the auth helper script).

**Two sets of credentials are needed** (obtained by a prep team member registering at `https://xaa.dev/developer/register`):

- **IDP client credentials:** `client_id` and `client_secret` — for OIDC login and token exchange
- **Resource client credentials:** `{client_id}-at-todo0` and its secret — for the JWT Bearer grant against `auth.resource.xaa.dev`

The MCP SDK's `withCrossAppAccess()` middleware may accept the same credentials for both `idpClientId`/`idpClientSecret` and `mcpClientId`/`mcpClientSecret`, handling the `-at-todo0` convention internally. If not, both pairs must be provided separately.

---

## Detailed Implementation Tasks

### 1. Fork and Extend mcp-ollama-agent

The agent source is at `https://github.com/ausboss/mcp-ollama-agent`. The workshop container runs a version of this. You need to modify its source code. The key files are:

- `src/MCPClientManager.ts` — manages MCP server connections
- `src/ChatManager.ts` — manages the Ollama chat loop and tool execution

**1a. Add StreamableHTTPClientTransport support to MCPClientManager.ts (~50-80 LOC)**

Currently, `MCPClientManager` only creates `StdioClientTransport` (spawning local processes). Add a conditional path: when a server config entry has a `url` field (instead of `command`/`args`), create a `StreamableHTTPClientTransport` instead.

```typescript
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
```

The config detection logic:
- Has `command` field → stdio transport (existing path)
- Has `url` field → StreamableHTTP transport (new path)

**1b. Integrate XAA middleware for authenticated HTTP connections (~30-50 LOC)**

When the config entry has `auth.type: "xaa"`, build the `withCrossAppAccess` middleware and wrap `fetch`:

```typescript
import { withCrossAppAccess, applyMiddlewares } from "@modelcontextprotocol/sdk/client/auth.js";
// Note: exact import path may differ — check the MCP SDK source for where
// withCrossAppAccess and applyMiddlewares are exported from. They may be at:
// "@modelcontextprotocol/sdk/client/xaa.js" or similar.

const xaaMiddleware = withCrossAppAccess({
  idpUrl: serverConfig.auth.idpUrl,
  idToken: loadedIdToken,  // read from env var or file
  idpClientId: process.env.XAA_CLIENT_ID,
  idpClientSecret: process.env.XAA_CLIENT_SECRET,
  mcpClientId: process.env.XAA_RESOURCE_CLIENT_ID,
  mcpClientSecret: process.env.XAA_RESOURCE_CLIENT_SECRET,
});
const enhancedFetch = applyMiddlewares(xaaMiddleware)(fetch);
const transport = new StreamableHTTPClientTransport(new URL(serverConfig.url), {
  fetch: enhancedFetch,
});
```

**Important:** The `withCrossAppAccess` middleware may be in the `@modelcontextprotocol/client` package (PR #1328 area) rather than the main SDK. Check the npm package `@modelcontextprotocol/sdk` at the latest version for its actual export location. If `withCrossAppAccess` is not yet released in the SDK, implement the token exchange manually:

- POST to `idpUrl/token` with `grant_type=urn:ietf:params:oauth:grant-type:token-exchange`, `subject_token=<idToken>`, `subject_token_type=urn:ietf:params:oauth:token-type:id-token`, `requested_token_type=urn:ietf:params:oauth:token-type:oauth-id-jag+jwt`, `audience=https://auth.resource.xaa.dev` → get ID-JAG
- POST to `https://auth.resource.xaa.dev/token` with `grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer`, `assertion=<id-jag>`, `scope=todos.read` → get access token
- Wrap fetch to add `Authorization: Bearer <access_token>` header

**1c. Add MCP Resource support (~80-120 LOC)**

This is the most architecturally significant change. The mcp-ollama-agent only uses `listTools()`/`callTool()`. The Todo0 server exposes resources, not tools. The MCP SDK `Client` class already has `listResources()` and `readResource()` — they just aren't used.

**In MCPClientManager.ts:**

After connecting to each server, call `client.listResources()` in addition to `client.listTools()`. For each discovered resource, generate a synthetic tool definition that the LLM can call:

```typescript
// After connecting and listing tools, also list resources
const resourcesResult = await client.listResources();
const syntheticTools = resourcesResult.resources.map(resource => ({
  name: `resource_${sanitizeName(resource.uri)}`,
  description: resource.description || `Read resource: ${resource.uri}`,
  inputSchema: {
    type: "object" as const,
    properties: {},
    required: [],
  },
  // Store metadata for routing
  _isResource: true,
  _resourceUri: resource.uri,
  _serverName: serverName,
}));
```

Store a mapping from synthetic tool names to their resource URIs and server names. Expose a method to check if a tool name is actually a resource, and a method to call `readResource()` on the correct client.

**In ChatManager.ts:**

When routing tool calls, check the mapping: if the tool name corresponds to a synthetic resource-tool, call `readResource({ uri })` instead of `callTool()`. Format the response (which returns `contents[]` with `text` or `blob` fields) to match the format `callTool()` returns (which has `content[]` with `text` fields).

**Why synthetic tools instead of injecting resources into context at startup:** The LLM decides when to fetch data, keeping the interaction conversational. The user asks "what are my todos?" and the LLM calls `resource_todo0_todos`. With startup injection, the data would be stale and waste context window. The synthetic-tools approach is the pragmatic bridge for an LLM-driven agent that lacks an application UI layer for resource browsing.

**1d. Update the system prompt in ChatManager.ts**

Add awareness of the Todo0 resource-tools to the system prompt so the LLM knows when to use them:

```
You also have access to a remote Todo application (Todo0) via the xaa.dev platform.
You can read the user's todos, check completed or incomplete items, and view statistics.
Use the resource_todo0_* tools to access this data when the user asks about their todos.
```

**1e. Update MCP Client capabilities declaration**

When constructing the MCP `Client`, ensure capabilities include `resources: {}` in addition to `tools: {}`:

```typescript
const client = new Client(
  { name: "mcp-ollama-agent", version: "1.0.0" },
  { capabilities: { tools: {}, resources: {} } }
);
```

**1f. Bump @modelcontextprotocol/sdk dependency**

The agent may be pinned to an older SDK version that lacks `StreamableHTTPClientTransport` or `withCrossAppAccess`. Update `package.json` to the latest SDK version. Run `npm install` and verify nothing breaks for the existing stdio path.

---

### 2. Create the OIDCTODO0 MCP Config File

Create `docker-compose-lab-mcp-config-files/OIDCTODO0.mcp-config.json`:

```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres",
               "postgresql://lab_user:lab_password@postgres:5432/workshop_db"]
    },
    "todo0": {
      "transport": "streamable-http",
      "url": "https://mcp.xaa.dev/mcp",
      "auth": {
        "type": "xaa",
        "idpUrl": "https://idp.xaa.dev",
        "authServerUrl": "https://auth.resource.xaa.dev",
        "audience": "https://mcp.xaa.dev/mcp",
        "scopes": ["todos.read"]
      }
    }
  },
  "ollama": {
    "host": "http://ollama:11434",
    "model": "llama3.2:1b"
  }
}
```

**Notes:**
- The `postgres` entry should match exactly what the existing `WORKING.mcp-config.json` has (copy it verbatim — the credentials, hostname, database name may differ from what I've shown above).
- The `todo0` entry does NOT contain client credentials or the ID token — those are read from environment variables at runtime.
- Also create `claude-desktop-lab-mcp-config-files/OIDCTODO0.mcp-config.json` if a Claude Desktop variant makes sense. It may not, since the auth script and forked agent code are Docker-specific. If you skip it, note in the README that the OIDCTODO0 config is Docker Compose only.

---

### 3. Create the Auth Helper Script

Create `get-xaa-token.sh` in the repo root. This script runs **inside the client container** and handles steps 1-2 of the XAA auth flow (OIDC login → ID token). The `withCrossAppAccess` middleware (or manual token exchange in the agent code) handles steps 3-5.

The script should:

1. Load `XAA_CLIENT_ID`, `XAA_CLIENT_SECRET`, and `XAA_REDIRECT_URI` from the `.env` file or environment
2. Generate a PKCE code_verifier (random 128-char string) and code_challenge (SHA-256 + base64url)
3. Generate a random `state` parameter
4. Print the full authorization URL for the student to open in their local browser:
   ```
   https://idp.xaa.dev/authorize?
     client_id=<XAA_CLIENT_ID>
     &redirect_uri=<XAA_REDIRECT_URI>
     &response_type=code
     &scope=openid%20profile%20email
     &code_challenge=<generated>
     &code_challenge_method=S256
     &state=<generated>
   ```
5. Prompt the student to paste the authorization code from their browser URL bar
6. Exchange the code for tokens via curl POST to `https://idp.xaa.dev/token`:
   ```
   grant_type=authorization_code
   &code=<pasted_code>
   &redirect_uri=<XAA_REDIRECT_URI>
   &code_verifier=<generated>
   ```
   With HTTP Basic auth: `Authorization: Basic base64(client_id:client_secret)`
7. Extract the `id_token` from the JSON response (use `jq`)
8. Write the ID token to a known location:
   - `export XAA_ID_TOKEN=<token>` appended to a file the agent sources on startup, OR
   - Write to `~/.xaa/id-token` file that the agent reads

The script should clearly explain to the student that:
- The browser redirect to `localhost:3000/callback` will FAIL (nothing is listening there) — that's expected
- They need to copy the `code` parameter from the browser URL bar
- The playground IDP accepts any email and sends a one-time code for verification

**Important:** Ensure `curl`, `jq`, `openssl` (for SHA-256), and `base64` are available in the container image. If not, add them to the Dockerfile or write the PKCE generation in Node.js instead.

Make the script executable: `chmod +x get-xaa-token.sh`

---

### 4. Create the .env Template

Create `.env.xaa.example` in the repo root:

```bash
# XAA / OIDC Configuration for Todo0 MCP Server
# These credentials are obtained by registering at https://xaa.dev/developer/register
# Registration is a one-time setup step performed by the workshop prep team.

# IDP Client Credentials (from xaa.dev registration)
XAA_CLIENT_ID=your-registered-client-id
XAA_CLIENT_SECRET=your-registered-client-secret

# Resource Client Credentials (may be auto-generated as {client_id}-at-todo0)
# If registration only returned one set of credentials, these may be the same as above.
XAA_RESOURCE_CLIENT_ID=your-client-id-at-todo0
XAA_RESOURCE_CLIENT_SECRET=your-resource-client-secret

# Redirect URI (must match what was registered at xaa.dev)
XAA_REDIRECT_URI=http://localhost:3000/callback

# This is populated by the get-xaa-token.sh script. Do not set manually.
# XAA_ID_TOKEN=
```

---

### 5. Update docker-compose.yml

The client container may need modifications:

- Ensure the container has outbound HTTPS access to `idp.xaa.dev`, `auth.resource.xaa.dev`, and `mcp.xaa.dev`. If network restrictions exist, they must be relaxed for this integration.
- Ensure the `.env.xaa` file (the real one, not the example) is mounted or loaded. This may mean adding `env_file: .env.xaa` to the client service, or ensuring the existing `.env` loading mechanism includes the XAA variables.
- Ensure `curl`, `jq`, and `openssl` are available in the container image. If the Dockerfile for the client container doesn't include them, they need to be added.
- Ensure the `get-xaa-token.sh` script is copied into the container (or mounted via volume).

---

### 6. Update the Cheatsheet

Add a new section to the cheatsheet (in `cheatsheet/`) for the OIDC/Todo0 step. Follow the exact style of the existing cheatsheet sections — copy-paste friendly, minimal explanation, clear steps.

The cheatsheet section should include:

**Step 1 — Exit current chat:**
```
exit
```

**Step 2 — Switch to the OIDC Todo0 config:**
```
cp docker-compose-lab-mcp-config-files/OIDCTODO0.mcp-config.json mcp-config.json
```
(Adjust the exact copy command to match how previous config swaps are documented in the existing cheatsheet.)

**Step 3 — Authenticate with xaa.dev:**
```
./get-xaa-token.sh
```
Then include a brief note: "Open the printed URL in your browser. Log in (any email works — the playground sends a verification code). After login, your browser will try to redirect to localhost and fail. Copy the `code=` value from the URL bar and paste it back into the terminal."

**Step 4 — Start the agent:**
```
npm start
```

**Step 5 — Try these prompts:**
```
What are my todos?
How many todos have I completed?
Show me my incomplete todos.
What are my todo statistics?
```

---

### 7. Update README.md

Update the repo's `README.md`:

- In the "MCP Configuration Files" section, mention that each config directory now contains `WORKING`, `SECRETWRAPPED`, and `OIDCTODO0` versions.
- Add a brief description: "The `OIDCTODO0` configuration connects to a remote Todo0 MCP server at xaa.dev using OIDC/XAA (Cross App Access) authentication, demonstrating authenticated access to a third-party MCP resource server."
- In the "Useful Primary Sources" section, add links to:
  - `https://xaa.dev/docs/mcp-overview` — XAA MCP integration documentation
  - `https://xaa.dev/docs/mcp-auth` — XAA MCP authentication flow
  - `https://xaa.dev/docs/mcp-quickstart` — XAA MCP quickstart guide
- If there's a "Repository Structure" section, add `get-xaa-token.sh` and `.env.xaa.example` to the listing.
- Note that the OIDCTODO0 step requires outbound internet access and a one-time registration at `xaa.dev/developer/register` by the prep team.

---

### 8. Key Technical Details to Get Right

**MCP SDK version:** The agent needs `@modelcontextprotocol/sdk` at a version that includes `StreamableHTTPClientTransport`. This was added in SDK v1.x. Check the latest published version on npm and pin to it.

**`withCrossAppAccess` availability:** This middleware may be in the SDK proper or in a separate `@modelcontextprotocol/client` package. Search the MCP TypeScript SDK GitHub repo (`https://github.com/modelcontextprotocol/typescript-sdk`) for `withCrossAppAccess` to find its exact export path. If it's only in an unmerged PR (#1328), implement the two-step token exchange manually instead — do NOT depend on unreleased code for a workshop.

**ID Token storage:** The auth script produces an ID token. The agent code needs to read it. The simplest approach: the script writes to a file (e.g., `/tmp/.xaa-id-token`), and the agent reads that file path from an env var or convention. Do not put the token in a JSON config file — it's ephemeral (~1 hour TTL).

**Synthetic tool naming:** When generating tool names from resource URIs like `todo0://todos/completed`, sanitize them to valid function names. Example: `resource_todo0_todos_completed`. The LLM needs clear, readable names.

**Error handling:** If the ID token file is missing or expired when the agent starts, print a clear error: "No XAA ID token found. Run ./get-xaa-token.sh first." Do not crash silently.

**Ollama model:** The workshop uses `llama3.2:1b` — a very small model. The synthetic resource-tools have zero parameters (no arguments for the LLM to construct), which is ideal for small models. The LLM just needs to pick the right named function. Test that it works by checking that the LLM reliably selects `resource_todo0_todos` when asked "what are my todos?"

---

## What NOT to Change

- Do not rename or restructure `WORKING.mcp-config.json` or `SECRETWRAPPED.mcp-config.json`
- Do not reorganize the repo into "Phase 1 / Phase 2 / Phase 3" directories
- Do not change the Docker Compose service names or the fundamental container architecture
- Do not replace Ollama with a different LLM provider
- Do not change the existing Postgres MCP server configuration in any way
- Do not add a Claude Desktop variant of the OIDCTODO0 config unless it actually works with Claude Desktop (it likely doesn't — the auth script is terminal-based)

---

## Summary of Files to Create or Modify

**New files:**
- `docker-compose-lab-mcp-config-files/OIDCTODO0.mcp-config.json`
- `get-xaa-token.sh` (executable)
- `.env.xaa.example`

**Modified files in mcp-ollama-agent source (forked):**
- `src/MCPClientManager.ts` — add HTTP transport, XAA auth middleware, resource listing + synthetic tool generation
- `src/ChatManager.ts` — route synthetic resource-tools to `readResource()` instead of `callTool()`
- `package.json` — bump `@modelcontextprotocol/sdk` dependency, add any new deps

**Modified repo files:**
- `README.md` — document the new config variant and XAA integration
- `cheatsheet/` — add OIDCTODO0 steps (examine existing files in this directory and add to them in the same style)
- `docker-compose.yml` — ensure `.env.xaa` is loaded and container has necessary tools (curl, jq, openssl)

---

## Acceptance Criteria

When complete, a student in the `mcp-workshop-astrix-academy-2026-client-1` container should be able to:

1. Exit the current chat session
2. Copy `OIDCTODO0.mcp-config.json` into place as `mcp-config.json`
3. Run `./get-xaa-token.sh`, authenticate via their browser, paste the auth code
4. Run `npm start`
5. Type "What are my todos?" and receive a response containing their actual Todo0 data from `xaa.dev`
6. Type "What are my todo statistics?" and receive counts of total/completed/incomplete todos

The agent should simultaneously support both the local Postgres MCP server (stdio, tools) and the remote Todo0 MCP server (HTTP, resources-as-synthetic-tools). Existing WORKING and SECRETWRAPPED configs must continue to work unchanged.
