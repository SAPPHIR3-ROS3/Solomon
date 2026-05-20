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

### 1. `go install` (recommended)

Requires [Go](https://go.dev/) **1.25.0+** ([`go.mod`](go.mod)).

Latest:

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```

Pin a [GitHub tag](https://github.com/SAPPHIR3-ROS3/Solomon/tags):

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```

Ensure `$GOPATH/bin` (or `$GOBIN`) is on your `PATH`.

### 2. Build from a clone

For contributors or local patches:

```bash
git clone https://github.com/SAPPHIR3-ROS3/Solomon.git
cd Solomon
make build
```

Produces `./solomon` (Unix/macOS) or `./solomon.exe` (Windows). See [Building and releases](docs/development/building-and-releases.md).

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
