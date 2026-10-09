# Installation reliability audit

This audit covers source installation (`make install`, `make hot-install`,
`make desktop-install`, and the Windows desktop build script), release
installers, coordinated updates, desktop provisioning, and daemon discovery.

## Verified defects and corrections

| Failure | Result before the fix | Corrected behavior |
| --- | --- | --- |
| Missing or damaged daemon state | A second daemon attempted to bind an occupied port | Live health metadata can restore state when the daemon identifies the same Solomon home |
| Concurrent daemon starts | Different listeners could overwrite the same state file | A per-home daemon lock prevents a second owner |
| Windows wildcard/loopback port sharing | A new daemon could bind while its health checks reached another listener | The daemon requests exclusive port ownership before binding, as described in [Microsoft's socket documentation](https://learn.microsoft.com/en-us/windows/win32/winsock/using-so-reuseaddr-and-so-exclusiveaddruse) |
| Stale PID or unrelated HTTP service | CLI health checks accepted any HTTP 200; startup could forcibly terminate a stale PID | Health checks validate daemon metadata; startup never kills a PID obtained from stale state |
| Failed server command | Printed an error but exited successfully | Errors reach the CLI exit status; startup detects early child exit |
| Failed build or dependency download | Source installation had already stopped the service | CLI, native desktop, browser dependencies, and Cursor dependencies are prepared before shutdown |
| Failed source deployment or restart | Could leave a partially replaced CLI and desktop | Independent backups cover CLI, desktop, desktop marker, and deployed Cursor integration until readiness is verified |
| Failed release rollback on Windows | Restoring over an existing executable could fail; fixed backup names could collide | Replacement uses distinct backups and restores through the same replacement operation |
| Failed desktop registration | Previous desktop and installation marker could be lost | Desktop provisioning restores both after a registration failure |
| Cursor npm failure | The previous integration was deleted first | Production dependencies are staged before replacing the integration directory |
| macOS source install | Used the Linux desktop path and installer | Builds and deploys the complete `Solomon.app` bundle and restarts its actual executable |
| macOS shell utilities | Required GNU `sort -V` and `sha256sum` | Numeric version comparison uses awk; hashing supports macOS `shasum -a 256` |
| Checksum download failure | Could silently install an unverified release | CLI and desktop downloads require checksums; errors preserve the installed version |
| Unix pinned install command | Assigned the version to curl rather than bash | The installer receives `SOLOMON_VERSION` |
| Installer config update | Removed matching keys inside unrelated TOML tables | Only top-level scalars are modified |
| Go archive extraction failure | Deleted the existing toolchain before extraction | Extracts and validates the replacement before moving the existing toolchain |
| Release reinstall with active Solomon | Could leave old processes serving the new binary's installation | Standalone installers refuse active installations and direct users to coordinated upgrade |
| Runtime address or mode changed during update | Clients could reconnect to another port or a release could keep serving an old dev frontend | Handoff preserves the active port; release upgrades use production UI, source installs preserve the prior mode |
| Nonstandard install directory | Updater could replace another CLI in GOPATH/bin | Explicit `SOLOMON_BINARY` or the running installed CLI determines the destination |
| Timeout during client shutdown | Recovery could forget a client whose stop request had already succeeded, or restore files while a process was still exiting | Requested clients are recorded immediately; recovery waits independently for pending shutdowns and leaves untouched binaries in place when deployment never started |
| Empty or incomplete desktop artifacts | A local directory or empty executable could replace the desktop; an invalid bundle could be registered | Local builds and extracted macOS executables must be nonempty regular executable files; empty release downloads fail before replacement |
| Cached browser with missing, broken, or outdated Node | The installer reported the browser ready despite an unusable runtime | Unix and PowerShell readiness checks verify Node before accepting cached browser files |

## Regression checks

The tests in [installation recovery](../../test/installation_recovery_test.go),
[installation scripts](../../test/installation_scripts_test.go),
[preparation failure](../../test/installation_prepare_test.go),
[hot install](../../test/hot_install_test.go),
[runtime update](../../test/lifecycle_update_test.go), and
[desktop installation](../../test/desktop_install_test.go) exercise isolated
homes, ports, executables, and mock downloads. They never replace the user's
installation or terminate the user's daemon.

The release workflow runs the Go tests on Windows, Linux, and macOS. Linux also
runs the race detector. Shell tests exercise failed downloads, checksum mismatch,
paths containing spaces, table preservation, and version comparisons without
GNU sort. Desktop fixtures cover both executable files and macOS bundles.

### Local verification (2026-10-09)

| Environment | Result |
| --- | --- |
| Windows | Complete Go suite passed in an isolated source copy; native desktop build passed |
| Linux (Arch WSL) | Complete Go suite with race detection passed; final handoff regression checks also passed |
| macOS | Go cross compilation passed for amd64 and arm64; arm64 vet passed; native execution remains unverified |
| GUI and Cursor | All 43 GUI tests and all 89 Cursor tests passed |

Verification uses isolated source copies and temporary installations so the
user's running daemon and local changes remain available. The publication
checks also exercise the exact installation changes staged for the commit.

A second review repeated the complete Windows suite (86 seconds), Linux
installation and coordinated-update tests with race detection, macOS cross
compilation for both architectures, vet, and documentation checks. It added a
real-process regression that cancels the update after a client receives its stop
request, plus malformed desktop and cached-browser runtime checks. Native
macOS execution remains pending.

## Operational limits

- Native macOS execution requires a macOS runner. Cross compilation and shell
  regression tests do not verify Launch Services, signing, or GUI startup.
- Linux desktop builds require GTK3 and WebKitGTK 4.1 development libraries;
  installed desktops require their runtime libraries. Missing build dependencies
  now fail preparation while the current daemon remains available.
- Direct `go install` is outside the coordinator. For a running installation,
  use `solomon upgrade` or a source Make installation.
- Automatic recovery of a missing state file requires the new daemon's `home`
  metadata. Older daemons need their existing state file or manual recovery.
- A forceful OS termination or power loss can interrupt a transaction. Backups
  are retained when rollback cannot safely complete, and their paths are reported.
  Registry/menu side effects and user configuration are not a database transaction.
- The installers still depend on external Go, Node, npm, browser, and release
  services. Preparation errors are surfaced instead of reporting a successful
  install or stopping the current daemon first.
