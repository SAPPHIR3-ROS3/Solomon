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

### Manual GitHub release (current process)

1. Create and push a semver tag on `main`, e.g. `v0.1.0`.
2. On GitHub: **Releases → Draft a new release** → pick the tag → write notes → **Publish release**.

You can leave **Set as a pre-release** unchecked in the UI: for tags matching `v0.*`, [.github/workflows/release-prerelease.yml](../../.github/workflows/release-prerelease.yml) sets `prerelease: true` automatically after publish.

When you ship a stable **v1+** release, that workflow does not run (remove or extend it if your policy changes).

`go install` picks up the tag:

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@v0.1.0
```

There are no prebuilt release binaries yet; install is via Go module proxy + tag.

### CI and calendar tags

Push and pull requests run vet, test, and build ([release.yml](../../.github/workflows/release.yml)).

Optional: **Actions → Release → Run workflow** creates an automated calendar tag (`vYYYY.MMDD.N`) via `workflow_dispatch` — separate from semver early releases.

## See also

- [Project README](../../README.md) — install and requirements
- [Startup and CLI](../architecture/startup-and-cli.md)
