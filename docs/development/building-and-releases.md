# Building and releases

## Development checks

From the repository root:

```bash
go vet ./...
go test ./... -count=1
go build ./cmd/solomon
```

Same checks as [.github/workflows/release.yml](../../.github/workflows/release.yml).

## Local build via Makefile

```bash
make build
```

Produces `solomon` (Unix/macOS) or `solomon.exe` (Windows). `CGO_ENABLED=0` per [Makefile](../../Makefile).

## Install from module path

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```

Ensure the remote tag you want exists.

## Releases

Tags are created manually via `workflow_dispatch` on the release workflow. Browse **Tags** on GitHub for versions. That workflow does not publish prebuilt release binaries unless extended.

CI verifies vet, test, and build on push; see the workflow file for triggers.

## See also

- [Project README](../../README.md) — install and requirements
- [Startup and CLI](../architecture/startup-and-cli.md)
