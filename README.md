# MCP Workshop Lab Materials

This repository contains the lab materials for the Astrix MCP Workshop. Choose your lab environment based on your setup preferences.

## Useful Primary Sources
These are all sources of documentation that go deeper into many aspects of things we will cover briefly in the workshop. They are gathered here for convenience, but don't represent the full scope of all materials that went into creating the labs and other workshop materials.
- [Introduction to Model Context Protocol](https://modelcontextprotocol.io/docs/getting-started/intro) - A foundational guide for newcomers, covering core concepts, installation prerequisites, and the initial steps to connect your first server to a client like Claude Desktop.
- [Manage MCP Servers in VS Code](https://code.visualstudio.com/docs/copilot/customization/mcp-servers#_manage-installed-mcp-servers) - A practical manual for VS Code users, detailing how to configure, manage, and troubleshoot MCP servers specifically within the GitHub Copilot environment for streamlined coding workflows.
- [Official MCP Servers Repository](https://github.com/modelcontextprotocol/servers) - The official repository containing reference implementations of various MCP servers, serving as both a library of ready-to-use tools and a gold-standard code reference for building your own.
- [Building a Custom MCP Server](https://modelcontextprotocol.io/docs/develop/build-server) - A technical tutorial for developers that walks through the lifecycle of creating a custom MCP server, including project initialization, defining tools, resources, and handling client requests.
- [How to Connect Data to AI Apps](https://www.builder.io/blog/mcp-server) - A narrative-driven tutorial that demonstrates practical application, guiding you through building a custom server to bridge proprietary data sources with AI applications for real-world use cases.

## Lab Choices

You have two main options for participating in the labs:

### Option 1: Astrix-Supplied Lab
Use the hosted lab environment provided by Astrix (runs Docker Compose on a hosted EC2 instance).

### Option 2: Bring Your Own (BYO)
Set up your own lab environment. Three sub-options are available:
- **Claude Desktop + AccuWeather API**: Use Claude Desktop with the AccuWeather developer API
- **Docker Compose**: Use Docker Compose with the Astrix lab configuration
- **Custom Tools**: Bring your own MCP-compatible tools (no advance instructions provided)

## Repository Structure

### Main Directory Files
The files in the main directory (`docker-compose.yml`, `pg-init/`, `pull-ollama-model.sh`, `get-xaa-token.sh`, `.env.xaa.example`) are used by both:
- Astrix-supplied labs (hosted Docker Compose environment)
- BYO Docker Compose labs

### MCP Configuration Files
Two sets of MCP configuration files are provided:

- **`claude-desktop-lab-mcp-config-files/`**: Configuration files for Claude Desktop labs
  - Used by: Claude Desktop + AccuWeather API 

- **`docker-compose-lab-mcp-config-files/`**: Configuration files for Docker Compose labs
  - Used by: Docker Compose labs (both Astrix-supplied and BYO)

The `docker-compose-lab-mcp-config-files/` directory contains three variants:
- **`WORKING`** — hardcoded credentials, used in the first phase of the workshop
- **`SECRETWRAPPED`** — credentials fetched from AWS Secrets Manager via mcp-secret-wrapper
- **`OIDCTODO0`** — connects to a remote Todo0 MCP server at xaa.dev using OIDC/XAA (Cross App Access) authentication — demonstrates authenticated access to a third-party MCP resource server

The `claude-desktop-lab-mcp-config-files/` directory contains `WORKING` and `SECRETWRAPPED` variants only (there is no Claude Desktop variant of OIDCTODO0; the auth script is terminal-based and container-specific).

### XAA/OIDC Prerequisites (OIDCTODO0 step)
- Requires outbound HTTPS access to `idp.xaa.dev`, `auth.resource.xaa.dev`, and `mcp.xaa.dev`
- Requires one-time prep-team registration at `xaa.dev/developer/register` to obtain client credentials
- See `.env.xaa.example` for the env vars that must be set before running `get-xaa-token.sh`

NOTE: DO NOT USE ANY OF THE DOCKER IMAGES IN A PRODUCTION SETTING. These have been built with the express purpose of demonstrating an insecure, bad state configuration. There is no data worth stealing in them right now, but if you put some there, you're asking for trouble!
