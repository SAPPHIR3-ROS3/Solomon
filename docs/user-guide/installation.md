# Installation and PATH

How to install Solomon and ensure `solomon` is on your shell `PATH`.

## Requirements

- [Go](https://go.dev/) **1.25.0+** ([`go.mod`](../../go.mod))
- Network access for `go install` or the install script

The standard installer does **not** require Node.js. Node/npm are only needed for the optional Cursor integration.

## Install script (recommended)

Installs Go **1.25.0+** if needed, ensures `make` is available, configures your shell `PATH`, and runs `go install`.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.sh | bash
```

From a clone: `./scripts/install.sh`

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.ps1 | iex
```

From a clone: `powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1`

Reload the terminal, then run `solomon version`.

## `go install` (manual)

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@latest
```

Pin a [release tag](https://github.com/SAPPHIR3-ROS3/Solomon/tags): `@v2026.527.2`

If `solomon` is not found after install, the binary is in `$(go env GOPATH)/bin` — see [Binary location](#binary-location) below.

## Build from a clone

```bash
git clone https://github.com/SAPPHIR3-ROS3/Solomon.git
cd Solomon
make build
```

Produces `./solomon` (Unix/macOS) or `./solomon.exe` (Windows). Release workflow and CI checks: [Building and releases](../development/building-and-releases.md).

## Verify install

```bash
solomon version
```

## Binary location

`go install` places the binary in `$(go env GOPATH)/bin` — by default `~/go/bin` on macOS and Linux, and `%USERPROFILE%\go\bin` on Windows. Go does **not** add this directory to your PATH automatically.

**Check that the binary exists:**

```bash
# macOS / Linux
ls "$(go env GOPATH)/bin/solomon"
```

```powershell
# Windows (PowerShell)
Test-Path "$(go env GOPATH)\bin\solomon.exe"
```

If the file is there but `solomon` is not found, configure PATH below.

## macOS / Linux

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

## Windows

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

## Alternative: set `GOBIN`

If you already have a directory on your PATH (e.g. `~/.local/bin` on macOS/Linux), you can tell Go to install binaries there instead:

```bash
go env -w GOBIN="$HOME/.local/bin"
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@latest
```

On Windows, use a path already on your PATH (e.g. `%USERPROFILE%\.local\bin`) and set `GOBIN` accordingly before re-running `go install`.

## See also

- [Quickstart](usage-and-commands.md#quickstart)
- [Building and releases](../development/building-and-releases.md)
- [Overview](../architecture/overview.md)
