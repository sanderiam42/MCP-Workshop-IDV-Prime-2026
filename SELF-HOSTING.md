# Self-Hosting the Lab

This document walks through standing up the lab environment on your own machine instead of the AWS-hosted instance. When you finish the steps here, you will be at the point in [README.md](README.md) where the lab walkthrough begins — sitting at a shell prompt inside the client container, ready to clone `mcp-ollama-agent` and start the chat UI.

This guide assumes working familiarity with Docker, Docker Compose, the shell (or PowerShell), and editing config files. It does not re-explain those tools.

## System Requirements

These are the resources the lab actually uses once everything is running. Below the minimums things will still start, but inference will be slow, the model pull may stall, and containers may be killed under memory pressure.

| Resource | Minimum | Recommended |
|---|---|---|
| RAM allocated to Docker | 8 GB | 12 GB+ |
| Free disk space | 20 GB | 30 GB+ |
| CPU | 4 cores | 8 cores, or Apple Silicon |
| Network | reachable egress to `hub.docker.com`, `ollama.com`, `npmjs.org`, `github.com` | same |

On Mac and Windows, Docker Desktop runs in a Linux VM, so "RAM allocated to Docker" is a setting you configure in Docker Desktop — not just the RAM on your machine. The default allocation is typically lower than the minimum above. Raise it before starting the lab.

The Ollama model used in the lab (`ministral-3:3b`) is roughly 2 GB on disk and uses 3–4 GB of RAM while serving. The Postgres, Node, and three Go services together use another ~1–2 GB. Build artifacts and image layers account for most of the disk requirement.

GPU acceleration is not required and is not configured in the compose file. Inference runs on CPU. On Apple Silicon (M1 or newer) this is comfortable. On older Intel/AMD CPUs it will be noticeably slower but functional.

## Choose Your Path

The setup steps diverge based on operating system and your tooling preferences. Pick one path and follow only that section through to the end of "Start the Lab" — then return to the shared sections below.

- **[Path A — Mac with Homebrew](#path-a--mac-with-homebrew)**: Apple Silicon or Intel Mac, you already use Homebrew or are willing to install it.
- **[Path B — Mac without Homebrew](#path-b--mac-without-homebrew)**: Apple Silicon or Intel Mac, prefer Apple-provided or vendor installers over Homebrew.
- **[Path C — Windows with WSL2](#path-c--windows-with-wsl2)**: Windows 10/11, comfortable running Linux through WSL2. The closest experience to the hosted lab.
- **[Path D — Windows without WSL2](#path-d--windows-without-wsl2)**: Windows 10/11, want to avoid WSL entirely. Uses Docker Desktop with the Hyper-V backend and PowerShell. Requires manual file edits in place of the bash helper scripts.

Paths A, B, and C end at a Unix shell running `./start-lab.sh`. Path D ends at PowerShell running `docker compose up --build -d` directly. After that, all four paths converge at [After the Lab is Running](#after-the-lab-is-running).

---

## Path A — Mac with Homebrew

### A.1 Install prerequisites

1. Install Docker Desktop for Mac. After install, open Docker Desktop → Settings → Resources and raise the memory allocation to at least 8 GB.
2. Install Go and Git via Homebrew:
   ```bash
   brew install go git
   ```
3. Confirm the toolchain:
   ```bash
   docker version
   docker compose version
   go version
   git --version
   ```

### A.2 Clone the repository

```bash
git clone https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
cd MCP-Workshop-IDV-Prime-2026
```

All paths below are relative to the repo root.

### A.3 Build the XAA bridge binary

`start-lab.sh` copies a Linux binary (`xaa-mcp-stdio-linux-amd64`) into the client container so the XAA / IDJAG lab can use it. In the hosted lab the binary is downloaded from a private S3 bucket. Self-hosters do not have access to that bucket, so build it locally first:

```bash
make build-stdio-linux
```

This produces `./bin/xaa-mcp-stdio-linux-amd64`. The presence of that file causes `start-lab.sh` to skip the S3 download path. Building targets Linux/amd64 regardless of host architecture because the binary runs inside the client container, not on the host.

### A.4 Create `~/lab-config.env`

`start-lab.sh` requires this file. The hosted lab creates it automatically; you create it by hand:

```bash
cat > ~/lab-config.env <<'EOF'
OLLAMA_MODEL=ministral-3:3b
OLLAMA_CONTAINER=mcp-workshop-idv-prime-2026-ollama-1
WORKSHOP_REPO=https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
WORKSHOP_REPO_DIR=MCP-Workshop-IDV-Prime-2026
EOF
```

Notes on these values are in [Values in `lab-config.env`](#values-in-lab-configenv) below.

### A.5 Start the lab

```bash
source ~/lab-config.env
./start-lab.sh --build -d
```

Skip to [After the Lab is Running](#after-the-lab-is-running).

---

## Path B — Mac without Homebrew

### B.1 Install prerequisites

1. Download and install **Docker Desktop for Mac** from `https://www.docker.com/products/docker-desktop/`. After install, open Docker Desktop → Settings → Resources and raise the memory allocation to at least 8 GB.
2. Install **Git** via Apple's Command Line Tools:
   ```bash
   xcode-select --install
   ```
   Click through the dialog that appears. This also gives you `make` and a system compiler.
3. Install **Go** from the official installer at `https://go.dev/dl/`. Download the macOS `.pkg` for your architecture (`arm64` for Apple Silicon, `amd64` for Intel) and run it. The installer puts `go` on `PATH` automatically.
4. Open a new terminal (so `PATH` updates take effect) and confirm the toolchain:
   ```bash
   docker version
   docker compose version
   go version
   git --version
   ```

### B.2 Clone the repository

```bash
git clone https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
cd MCP-Workshop-IDV-Prime-2026
```

### B.3 Build the XAA bridge binary

Same as Path A:

```bash
make build-stdio-linux
```

This produces `./bin/xaa-mcp-stdio-linux-amd64` and causes `start-lab.sh` to skip the S3 download. Building targets Linux/amd64 regardless of host architecture because the binary runs inside the client container.

### B.4 Create `~/lab-config.env`

```bash
cat > ~/lab-config.env <<'EOF'
OLLAMA_MODEL=ministral-3:3b
OLLAMA_CONTAINER=mcp-workshop-idv-prime-2026-ollama-1
WORKSHOP_REPO=https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
WORKSHOP_REPO_DIR=MCP-Workshop-IDV-Prime-2026
EOF
```

Notes on these values are in [Values in `lab-config.env`](#values-in-lab-configenv) below.

### B.5 Start the lab

```bash
source ~/lab-config.env
./start-lab.sh --build -d
```

Skip to [After the Lab is Running](#after-the-lab-is-running).

---

## Path C — Windows with WSL2

### C.1 Install prerequisites

1. Install WSL2 with an Ubuntu (or equivalent) distribution. From an elevated PowerShell:
   ```powershell
   wsl --install -d Ubuntu
   ```
   Reboot when prompted, then complete the Ubuntu first-run setup.
2. Install **Docker Desktop for Windows**. During install, enable the WSL2 backend. After install, open Docker Desktop → Settings → Resources → WSL Integration and enable integration with your Ubuntu distro. Under Settings → Resources, raise the memory allocation to at least 8 GB.
3. From inside the WSL2 Ubuntu shell, install Git and Go:
   ```bash
   sudo apt update
   sudo apt install -y git golang-go make
   ```
   If the Ubuntu repos do not yet have Go 1.25, install from the official tarball at `https://go.dev/dl/` and put `go` on `PATH`.
4. From inside the WSL2 shell, confirm the toolchain:
   ```bash
   docker version
   docker compose version
   go version
   git --version
   ```

Run all subsequent commands inside the WSL2 shell, not PowerShell or cmd.

### C.2 Clone the repository

Clone into the WSL2 filesystem (e.g. `~/projects/`), not into a Windows-mounted path like `/mnt/c/...`. Disk performance across the WSL boundary is significantly worse, and shell scripts can hit CRLF line-ending issues on NTFS.

```bash
git clone https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
cd MCP-Workshop-IDV-Prime-2026
```

### C.3 Build the XAA bridge binary

```bash
make build-stdio-linux
```

This produces `./bin/xaa-mcp-stdio-linux-amd64` and causes `start-lab.sh` to skip the S3 download.

### C.4 Create `~/lab-config.env`

```bash
cat > ~/lab-config.env <<'EOF'
OLLAMA_MODEL=ministral-3:3b
OLLAMA_CONTAINER=mcp-workshop-idv-prime-2026-ollama-1
WORKSHOP_REPO=https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
WORKSHOP_REPO_DIR=MCP-Workshop-IDV-Prime-2026
EOF
```

Notes on these values are in [Values in `lab-config.env`](#values-in-lab-configenv) below.

### C.5 Start the lab

```bash
source ~/lab-config.env
./start-lab.sh --build -d
```

Skip to [After the Lab is Running](#after-the-lab-is-running).

---

## Path D — Windows without WSL2

This path uses Docker Desktop with the Hyper-V backend and PowerShell. The bash helper scripts (`start-lab.sh`, `apply-lab-config.sh`) do not run natively on Windows, so this path replaces the script-driven substitutions with manual edits (or equivalent PowerShell commands).

You will still run the in-container helper script (`apply-lab-config.sh`) later — that one runs inside the Linux client container, which is fine. The host-side script (`start-lab.sh`) is what gets replaced here.

### D.1 Install prerequisites

1. Install **Docker Desktop for Windows** from `https://www.docker.com/products/docker-desktop/`. During install, leave the "Use WSL2" option **unchecked** and choose the Hyper-V backend instead. (If the installer requires WSL2 on your version of Windows, use Path C instead.) After install, open Docker Desktop → Settings → Resources and raise the memory allocation to at least 8 GB.
2. Install **Git for Windows** from `https://git-scm.com/download/win`. Accept the defaults. This also installs `git bash`, which you can ignore — these instructions use PowerShell.
3. Install **Go for Windows** from `https://go.dev/dl/`. Download the `.msi` for `windows-amd64` (or `windows-arm64`) and run it. The installer puts `go` on `PATH` automatically.
4. Open a new PowerShell window (so `PATH` updates take effect) and confirm the toolchain:
   ```powershell
   docker version
   docker compose version
   go version
   git --version
   ```

`make` is not strictly required on this path — the build step uses `go build` directly.

### D.2 Clone the repository

From PowerShell, in a directory where you keep code:

```powershell
git clone https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026
cd MCP-Workshop-IDV-Prime-2026
```

If `git` warns about converting line endings on checkout, ignore it — the files you will actually run live inside Linux containers, not on the Windows filesystem.

### D.3 Build the XAA bridge binary

The `make build-stdio-linux` target on the other paths runs a cross-compile to Linux/amd64. On Windows you can run the same `go build` directly. From PowerShell, in the repo root:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
cd xaa-demo
go build -o ..\bin\xaa-mcp-stdio-linux-amd64 .\cmd\xaa-mcp-stdio
cd ..
```

Confirm the file exists:

```powershell
ls .\bin\xaa-mcp-stdio-linux-amd64
```

You will copy this file into the running client container in step D.6.

### D.4 Substitute placeholders in the lab config files

The hosted lab and the Unix paths use `start-lab.sh` to run `sed` substitutions across several files before launching Docker Compose. On Windows you do the equivalent in PowerShell — or by hand in an editor — once. The placeholders and their values are:

| Placeholder | Replace with |
|---|---|
| `__OLLAMA_MODEL__` | `ministral-3:3b` |
| `__WORKSHOP_REPO__` | `https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026` |
| `__WORKSHOP_REPO_DIR__` | `MCP-Workshop-IDV-Prime-2026` |

The files that need substitution are:

- `docker-compose.yml`
- `docker-compose-lab-mcp-config-files\WORKING.mcp-config.json`
- `docker-compose-lab-mcp-config-files\SECRETWRAPPED.mcp-config.json`
- `docker-compose-lab-mcp-config-files\XAAIDJAG.mcp-config.json`

(The files under `cheatsheet\` contain additional placeholders that are only useful for the live workshop's copy-paste flow. You can ignore them for self-hosting.)

**Option 1 — PowerShell, scripted.** Run this from the repo root. It edits the four files in place:

```powershell
$model    = "ministral-3:3b"
$repo     = "https://github.com/sanderiam42/MCP-Workshop-IDV-Prime-2026"
$repoDir  = "MCP-Workshop-IDV-Prime-2026"

$files = @(
  "docker-compose.yml",
  "docker-compose-lab-mcp-config-files\WORKING.mcp-config.json",
  "docker-compose-lab-mcp-config-files\SECRETWRAPPED.mcp-config.json",
  "docker-compose-lab-mcp-config-files\XAAIDJAG.mcp-config.json"
)

foreach ($f in $files) {
  (Get-Content -Raw $f) `
    -replace "__WORKSHOP_REPO_DIR__", $repoDir `
    -replace "__WORKSHOP_REPO__",     $repo `
    -replace "__OLLAMA_MODEL__",      $model |
    Set-Content -NoNewline $f
}
```

Note the substitution order: `__WORKSHOP_REPO_DIR__` is replaced before `__WORKSHOP_REPO__` so that the prefix match does not eat the longer placeholder.

**Option 2 — manual editor.** Open each of the four files in your editor of choice and do a find/replace for each placeholder above with its value. The result is the same.

After substitution, the four files contain no `__SOMETHING__` placeholders.

### D.5 Confirm `.env`

The repo ships a `.env` at the root configured for container mode. Leave it as-is. Do not switch it to the local-mode block — local mode is only for running the Go services directly with `go run`, not for Docker Compose.

### D.6 Start Docker Compose and copy the binary into the client container

From PowerShell, in the repo root:

```powershell
docker compose up --build -d
```

First run pulls and builds images and pulls the Ollama model. Plan on 10–30 minutes depending on network and CPU. Watch the model pull in another PowerShell window:

```powershell
docker compose logs -f ollama-model-puller
```

You will see "SUCCESS!" when the model is downloaded.

Once the client container is running, find its full name and copy the XAA bridge binary into it:

```powershell
docker ps --format "{{.Names}}" | Select-String client
docker cp .\bin\xaa-mcp-stdio-linux-amd64 <client-container-name>:/root/
```

Replace `<client-container-name>` with whatever the previous command returned (typically something like `mcp-workshop-idv-prime-2026-client-1`).

Continue to [After the Lab is Running](#after-the-lab-is-running).

---

## After the Lab is Running

All four paths arrive here once Docker Compose has started and the XAA bridge binary is in place.

### Enter the client container

On Paths A, B, C:

```bash
source ~/lab-config.env
docker exec -it $CLIENT_CONTAINER bash
```

On Path D:

```powershell
docker exec -it <client-container-name> bash
```

From here, follow [README.md](README.md) starting at **Lab Walkthrough → Set Up the Chatbot (Agent)**. Everything from that point assumes you are at a shell prompt inside the client container. The lab steps inside the container are the same regardless of which path you took to get here.

### Lab 2 (SECRETWRAPPED) requires AWS

The SECRETWRAPPED config (`docker-compose-lab-mcp-config-files/SECRETWRAPPED.mcp-config.json`) instructs `mcp-secret-wrapper` to fetch the Postgres connection string from AWS Secrets Manager in `us-east-2`. To run Lab 2 against your own machine you need:

- An AWS account
- A secret in AWS Secrets Manager (any region; update `VAULT_REGION` in the config to match) containing the `DATABASE_URL` for the lab's Postgres service, formatted as `postgresql://demouser:demopass123@postgres:5432/demo`
- AWS credentials available inside the client container — either by mounting `~/.aws` into it via a Docker Compose override, or by exporting `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION` in the shell before running `npm start`
- The secret's ARN pasted into `mcp-config.json` in place of `{PASTE_THE_NEW_SECRET_ARN_HERE}`

If you do not want to set this up, you can skip Lab 2 and go directly from Lab 1 to Lab 3. The XAA / IDJAG lab (Lab 3) has no external service dependencies — the auth, resource, and requesting-app services all run in the local compose stack.

### Stopping and resetting

Stop the stack without removing data:

```bash
docker compose down
```

Remove containers, networks, and named volumes (this also deletes the pulled Ollama model and Postgres data):

```bash
docker compose down -v
```

Reset XAA demo state without touching containers (Paths A, B, C):

```bash
make reset-state
```

On Path D, the same effect is:

```powershell
Remove-Item -ErrorAction Ignore .\data\auth\auth-state.json,.\data\resource\resource-state.json,.\data\requesting-app\requesting-app-state.json
```

## Values in `lab-config.env`

For Paths A, B, and C, `~/lab-config.env` holds the values `start-lab.sh` substitutes into the config files. Notes on each:

- `OLLAMA_MODEL` — any model tag available on `ollama.com`. The lab is validated against `ministral-3:3b`. Larger models will increase memory and disk requirements.
- `OLLAMA_CONTAINER` — the container name Docker Compose assigns to the `ollama` service. Compose derives this from the directory name: `<lowercased-dir>-<service>-<index>`. If you cloned the repo under a different directory name, adjust accordingly, or set the project name explicitly with `COMPOSE_PROJECT_NAME` and use that as the prefix.
- `WORKSHOP_REPO` / `WORKSHOP_REPO_DIR` — the URL and directory name used inside the client container when the lab walkthrough re-clones the repo. Match what you cloned on the host.

`start-lab.sh` appends `CLIENT_CONTAINER=...` to this file on first run. That is expected.

Path D does not use `lab-config.env`. The same values are baked into the files directly in step D.4.

## Common Self-Host Issues

- **`start-lab.sh` exits with `lab-config.env not found`.** You did not create `~/lab-config.env`, or you ran the script from a shell that does not see the same `$HOME`. On Path C, run from inside the WSL2 shell, not PowerShell.
- **`docker exec` cannot find `$CLIENT_CONTAINER`.** Either the stack did not finish starting, or your `OLLAMA_CONTAINER` value in `lab-config.env` does not match the actual container name. Check with `docker ps --format '{{.Names}}'` and update the env file. Re-source it before running `docker exec`.
- **Ollama model pull times out or fails repeatedly.** The puller waits up to 2.5 minutes for the Ollama API to become reachable. On slower machines or first-time image builds it can take longer. Check `docker compose logs ollama` to confirm `ollama serve` is up, then re-run `docker compose up ollama-model-puller`.
- **Containers killed (exit code 137) during model load.** Docker is out of memory. Raise the memory allocation in Docker Desktop settings and restart Docker.
- **Shell scripts fail with `bad interpreter` or `\r: command not found` on Path C.** The repo was cloned with CRLF line endings. Re-clone inside WSL2 with `git config --global core.autocrlf input` set, or run `dos2unix` on the affected scripts.
- **`aws s3 cp` error from `start-lab.sh` (Paths A, B, C).** You did not pre-build the XAA binary. Run `make build-stdio-linux`, confirm `./bin/xaa-mcp-stdio-linux-amd64` exists, then re-run `start-lab.sh`.
- **`docker compose up` on Path D fails with errors about `__OLLAMA_MODEL__` or `__WORKSHOP_REPO__`.** Step D.4 was skipped or incomplete. Confirm none of the four listed files still contain `__SOMETHING__` placeholders.
- **`go build` on Path D fails with `cannot find package`.** You ran the command from the repo root instead of `xaa-demo/`. The relative paths in step D.3 assume you `cd xaa-demo` first.
