# ⚠️ EARLY RELEASE ⚠️

## PREVIEW SOFTWARE — NOT PRODUCTION-READY

**APIs, behavior, and on-disk formats may change without notice.**

Bring your own OpenAI-compatible endpoint · Expect rough edges · [Open an issue](https://github.com/SAPPHIR3-ROS3/Solomon/issues) with feedback

```
╔══════════════════════════════════════════════════════════════════════════╗
║                                                                          ║
║   S O L O M O N   ·   E A R L Y   R E L E A S E   ·   U S E   A T        ║
║   Y O U R   O W N   R I S K   ·   F E E D B A C K   W E L C O M E        ║
║                                                                          ║
╚══════════════════════════════════════════════════════════════════════════╝
```

# Solomon

Interactive terminal harness for LLMs over OpenAI-compatible APIs — project-aware sessions, skills, slash commands, planning, and tooling.

## Install

### 1. Install script (one command)

Installs Go **1.25.0+** if needed, configures your shell `PATH`, and runs `go install` for `solomon`.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.sh | bash
```

From a clone:

```bash
./scripts/install.sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.ps1 | iex
```

From a clone:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
```

Reload the terminal (or `source` your rc file), then run `solomon version`.

### 2. `go install` (manual)

Requires [Go](https://go.dev/) **1.25.0+** ([`go.mod`](go.mod)).

Latest:

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```

Pin a [GitHub tag](https://github.com/SAPPHIR3-ROS3/Solomon/tags):

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```


### 3. Build from a clone

For contributors or local patches:

```bash
git clone https://github.com/SAPPHIR3-ROS3/Solomon.git
cd Solomon
make build
```

Produces `./solomon` (Unix/macOS) or `./solomon.exe` (Windows). See [Building and releases](docs/development/building-and-releases.md).

## Add `solomon` to your PATH

After `go install`, the binary is placed in `$(go env GOPATH)/bin` — by default `~/go/bin` on macOS and Linux, and `%USERPROFILE%\go\bin` on Windows. Go does **not** add this directory to your PATH; configure it once below.

**Check that the binary exists:**

```bash
# macOS / Linux
ls "$(go env GOPATH)/bin/solomon"
```

```powershell
# Windows (PowerShell)
Test-Path "$(go env GOPATH)\bin\solomon.exe"
```

If the file is there but `solomon` is not found, follow the steps for your system.

### macOS / Linux

**zsh** (default on macOS) — add to `~/.zshrc`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Or append in one step:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```

**bash** — add to `~/.bashrc`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Or append in one step:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
source ~/.bashrc
```

**fish** — add to `~/.config/fish/config.fish`:

```fish
fish_add_path (go env GOPATH)/bin
```

Verify: `which solomon` then `solomon version`.

### Windows

**PowerShell profile** — add to `$PROFILE` (run `notepad $PROFILE` if the file does not exist yet):

```powershell
$env:Path += ";$(go env GOPATH)\bin"
```

**Current session only** (PowerShell):

```powershell
$env:Path += ";$(go env GOPATH)\bin"
```

**Permanent user PATH** (PowerShell, no profile file):

```powershell
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$(go env GOPATH)\bin", "User")
```

Restart the terminal, then run `solomon version`.

### Alternative: set `GOBIN`

If you already have a directory on your PATH (e.g. `~/.local/bin` on macOS/Linux), you can tell Go to install binaries there instead:

```bash
go env -w GOBIN="$HOME/.local/bin"
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```

On Windows, use a path already on your PATH (e.g. `%USERPROFILE%\.local\bin`) and set `GOBIN` accordingly before re-running `go install`.

## Quickstart

```bash
cd /path/to/your/project
solomon .
```

On first run, Solomon starts an **interactive setup** (provider URL, API key, model). Name and language are optional; provider credentials are required.

Then chat at the `You:` prompt, or send one message:

```bash
solomon exec hello
```

Reconfigure later: `/onboard` in the REPL. Backup config: `/configbackup`.

You need network access and credentials for an **OpenAI-compatible** HTTPS API (`base_url` + API key).

## Documentation

Full guides (configuration, commands, architecture, development): **[docs/](docs/README.md)**.

Startup flow diagram: [Startup and CLI](docs/architecture/startup-and-cli.md#startup-flow).

## License

[MIT License.](LICENSE)
