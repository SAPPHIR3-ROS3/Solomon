# Contributing

Use this guide for a first local change. Keep the change focused, include a regression test for behavior changes, and update the affected documentation in the same patch. Never commit credentials, personal configuration, session data or logs.

## Prerequisites and checkout

Install Git, Go at the version required by [go.mod](../../go.mod), Node.js 20+ with npm, and GNU Make. Native desktop development additionally requires the platform toolchain: GTK3/WebKitGTK 4.1 development packages on Linux, Xcode Command Line Tools on macOS, or the Windows desktop build prerequisites and WebView2. The CLI build does not require the Linux desktop headers.

```bash
git clone https://github.com/SAPPHIR3-ROS3/Solomon.git
cd Solomon
make build
```

`make build` prepares the Cursor integration, builds/stages the shared GUI, and compiles the CLI with version metadata. It can download existing locked dependencies. Do not add or update dependencies as an incidental part of a fix; discuss such changes separately with the maintainer.

Avoid `make install` or `make hot-install` just to compile a patch: these modify the installed application and coordinate a runtime restart. See [Building and releases](building-and-releases.md) for deployment and desktop requirements.

## Find the relevant code

| Area | Location | Guide |
|---|---|---|
| CLI and daemon commands | `cmd/solomon/` | [Startup and CLI](../architecture/startup-and-cli.md) |
| Agent turns and tools | `internal/agent/` | [Agent turn pipeline](../architecture/agent-turn-pipeline.md) |
| Providers and streaming | `internal/llm/` | [LLM layer](../architecture/llm-layer.md) |
| Daemon HTTP, SSE and PTYs | `internal/server/` | [Local server](../architecture/server.md) |
| Shared React frontend | `gui/src/` | [GUI side panels](gui-side-panels.md) |
| Native Wails client | `gui/desktop/` | [Building and releases](building-and-releases.md) |
| Documentation | `docs/` | [Documentation index](../README.md) |

Use the [package index](../architecture/package-index.md) for the complete Go package map and the [cookbook](cookbook.md) for feature-specific recipes.

## Local development

For frontend dependencies and a development daemon, from the repository root:

```bash
make gui-deps
solomon server status
```

If no daemon is running, start `solomon server start dev /absolute/path/to/Solomon/gui`. If one is running, finish active work before stopping or restarting it; the daemon owns chats and terminal processes. Open the URL printed by `solomon server status`. `make desktop-dev` connects Wails to the running development server.

For a production frontend check, run `make gui-build`; direct `go build` does not refresh staged frontend assets. Use `make desktop-build` when changing the native client.

## Checks before a pull request

After preparing the build assets with `make build`, run:

```bash
go vet ./...
go test ./... -count=1
make check-docs
npm --prefix gui test
npm --prefix gui run build
git diff --check
```

For Linux CI parity, also run `CGO_ENABLED=1 go test ./... -count=1 -race` with a working C compiler. If the Cursor sidecar changed, run `make cursor-proxy-test`. Browser/device suites have separate prerequisites; see [Testing](testing.md). Do not use live paid model calls as ordinary regression tests.

Place Go tests in top-level `test/`, GUI tests in `gui/tests/`, browser checks in `gui/e2e/`, and sidecar tests in `integrations/cursor/test/`. Prefer existing helpers, temporary directories and local HTTP fixtures. Keep files at or below 500 lines and split by responsibility when necessary.

## Documentation-only changes

```bash
make check-docs
git diff --check
```

The existing checker verifies local Markdown links, heading anchors and cited code paths; the package checker verifies coverage of Go packages in the index. Both already run in the release CI matrix. They do not verify remote URL availability or prove that prose matches runtime behavior. Review documented commands against source and preview Markdown manually, including fenced examples and tables. No new documentation framework is required.

## Pull request checklist

- Explain the problem, chosen scope and observable behavior in English.
- List checks actually run, their results, and any platform or environment limitations.
- Include regression coverage and relevant documentation links.
- Review the diff for unrelated changes, generated assets and sensitive data.
- Keep release/tag creation separate from ordinary contribution checks.

## See also

- [Testing](testing.md)
- [Building and releases](building-and-releases.md)
- [Troubleshooting](../user-guide/troubleshooting.md)
