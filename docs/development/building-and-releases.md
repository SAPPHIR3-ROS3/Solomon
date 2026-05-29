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

Module path: `github.com/SAPPHIR3-ROS3/Solomon/v2026`

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@latest
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@v2026.527.2
```

Release tags use calendar semver `vYYYY.MDD.N` (month×100+day in the middle component, e.g. `v2026.527.2` = 2026, May 27, revision 2).

## Yearly module path updates

When the calendar year advances, create a new major module path (e.g. `.../Solomon/v2027`) and update imports. Older paths remain installable via their existing tags (`@v2026.527.2` on `/v2026`, etc.).

## Releases

### CI and calendar tags

Push and pull requests run vet, test, and build ([release.yml](../../.github/workflows/release.yml)).

**Actions → Release → Run workflow** creates tag `vYYYY.MDD.N`, GitHub release assets, and a GitHub release.

Prebuilt binaries are attached per platform; the install scripts download those assets by default.

## See also

- [Project README](../../README.md) — install and requirements
- [Startup and CLI](../architecture/startup-and-cli.md)
