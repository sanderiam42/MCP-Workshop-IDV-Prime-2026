# MCP Workshop Lab

This repository is the hands-on lab environment for the **[AI + Identity Workshop at Identiverse 2026](https://identiverse.com/idv26/ai-identity-workshop/)** aka "The MCP Hackathon". Lab instances are pre-configured EC2 machines accessed via AWS CloudShell. All interaction with the LLM powered chat using MCP happens through a CLI chat interface.

## Credits for the contents of the lab

The chat & MCP elements of the lab leverage the work of [@ausboos](https://github.com/ausboss) from the project [mcp-ollama-agent](https://github.com/ausboss/mcp-ollama-agent). Many props go out to that person for making this all possible. 

The [XAA / IDJAG](https://datatracker.ietf.org/doc/draft-ietf-oauth-identity-assertion-authz-grant/) pieces come from the ongoing work from that project with special thanks to [Aaron Parecki](https://datatracker.ietf.org/person/aaron@parecki.com) of Okta for his help making some key choices. 

Last but not least, thanks to my partner in crime for this event [Nick Steele](https://github.com/nicksteele-oai), who wrote the [toy IdP with XAA support](https://github.com/sanderiam-astrix/MCP-Workshop-Astrix-Academy-2026/commit/fe6632ecac93b610d5fdbfb8f5fd9002b6b85f75) used in the labs. 

## What the Lab Covers

You will be dropped into a lab with a working LLM locally hosted on a container which is accessed via a chat UI that has tools ican access via MCP. The point of the lab is to alter the MCP configurations to access different tools in different ways, learning about the structure of MCP, how it's secured, and patterns you can use in your own IAM efforts to lock down MCP security specifically and LLM powered systems security in general. 

The lab walks through MCP security progressively, with each phase swapping only the MCP client configuration. There are three MCP client configs packaged in this repo which act as jumping off points for the 3 different phases of the lab work:

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
| `auth-server` | Demo OIDC / enterprise IdP |
| `resource-server` | XAA-protected MCP server — todo list |
| `requesting-app` | XAA-aware requesting app + client provisioning |

## Getting Started

***NOTE*** :: These instructions will assume the AWS hosted lab type. If you choose to self host these materials elsewhere, then it's assumed you have the skill to adjust the details of the technical steps to suit your needs in your hosting environment - whatever it may be.

### Connect to Your Lab Instance

From AWS CloudShell:

```bash
aws ssm start-session --target <your_instance_id> --document-name AWS-StartInteractiveCommand --parameters command="cd ~ && exec bash -l"
```

The value for `<your_instance_id>` will have been sent to you along with your hosted lab information. If you do not have that information, it means your resgitration for a hosted lab did not succeed on some level. Similarly, your ability to connect to the AWS account to access the CloudShell and other aspects of this hosted lab are gated on having completed the registration ahead of this point in time.

You'll know you've succeeded when you see something like this: 
```bash
~ $ aws ssm start-session --target i-06Edcr39437Fa --document-name AWS-StartInteractiveCommand --parameters command="cd ~ && exec bash -l"

Starting session with SessionId: teacher.lab-MCP-Workshop-Astrix-Academy-2026-xyshgelir33c2zo3nej5vd9cz4
[ssm-user@ip-172-31-38-175 ~]$ 
```
***NOTE*** :: The instance id used in the sample output is not in the right format, so don't worry if yours is longer. 

***NOTE*** :: The command prompt (i.e., `[ssm-user@ip-172-31-38-175 ~]$` in the sample output above) contians the IP address of the machine. Yours will be different numbers bu thte same format.

### Start the Lab

Run the following three commands in your new session:
```bash
source ~/lab-config.env
```
```bash
cd $WORKSHOP_REPO_DIR
```
```bash
./start-lab.sh --build -d
```

`start-lab.sh` reads `~/lab-config.env`, substitutes the configured model name and repo references into local files, starts all Docker Compose services, and copies the XAA MCP bridge binary into the client container. The Ollama model pulls automatically on first run — this may take a few minutes. It also writes `CLIENT_CONTAINER` back to `~/lab-config.env`.

When it finishes, re-source and enter the container:

```bash
source ~/lab-config.env
```
```bash
docker exec -it $CLIENT_CONTAINER bash
```

## Lab Walkthrough

All commands below run **inside the client container** unless noted otherwise.

### Set Up the Chatbot (Agent) (one-time)

```bash
cd ~ && git clone https://github.com/ausboss/mcp-ollama-agent && git clone $WORKSHOP_REPO
```
```bash
../$WORKSHOP_REPO_DIR/apply-lab-config.sh
```
```bash
chmod +x xaa-mcp-stdio-linux-amd64 && cd mcp-ollama-agent/
```
```bash
cp mcp-config.json ORIG.mcp-config.json
```
```bash
cp ../$WORKSHOP_REPO_DIR/docker-compose-lab-mcp-config-files/WORKING.mcp-config.json mcp-config.json
```
```bash
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
- The lab is using the `ministral-3:3b` model ([found here on huggingface](https://huggingface.co/mistralai/Ministral-3-3B-Instruct-2512))

### Running Outside the Workshop

To run this environment locally, use the browser UI, connect from Cursor or Codex, or step through the XAA token flow manually with curl — see [SUPPLEMENTARY.md](SUPPLEMENTARY.md).
