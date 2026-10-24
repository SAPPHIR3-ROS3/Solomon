# Upgrade verification

Audit performed on 2026-10-24 with Go 1.27.3. All runtime tests use temporary
installations, isolated `SOLOMON_HOME` directories and fixture release servers.
The installed application and its active daemon were not updated or restarted.

## Entry points and recovery

- **App update:** `test/update_api_test.go` exercises release discovery,
  download, explicit restart, duplicate actions, checksum/download failures,
  coordinator failure, retries and origin validation. `gui/e2e/updateButton.test.mjs`
  exercises the real React hook and button in Chromium, including progress,
  panel remounts, double clicks, retries and daemon disconnection/reconnection.
- **CLI and TUI:** command tests cover `solomon upgrade`, `/upgrade`, `/update`
  and `/autoupdate`. `test/tui_upgrade_integration_unix_test.go` runs actual
  fixture binaries, a daemon and a PTY. It verifies that explicit upgrade and
  startup autoupdate preserve the terminal identity and active chat.
- **Older coordinator:** the PTY fixture also simulates the older CLI-only
  replacement path. The new CLI repairs a stale desktop before starting its
  daemon. This checks protocol compatibility with that handoff, rather than
  claiming that every historical published binary was executed.
- **Split release transaction:** CLI and desktop are downloaded and checksummed
  before shutdown. Both executables and the desktop version marker participate
  in rollback. Tests cover unavailable network, invalid CLI/desktop checksum,
  missing desktop asset, cancellation, registration failure and startup failure.
  A release update also passes with an empty `PATH`, demonstrating that it needs
  no Go toolchain.
- **Standalone:** tests cover release asset inference, explicit layout selection,
  registered process discovery and desktop restart arguments. A saved layout
  survives modern and legacy TOML configuration round trips. Layout changes
  dispatch the common coordinator rather than replacing live files directly.
- **Source/hot install:** tests cover preparation before shutdown, Windows dev
  dependency locks, concurrent update rejection, cancellation, client restart
  and rollback. Rollback includes the standalone companion executable. Built
  desktop installation restores its previous files if registration fails.
- **Install scripts:** Unix fixtures cover older/current/forced Go toolchains,
  installation failures, checksums, cleanup, profiles and dependency setup.
  PowerShell fixtures run in a temporary portable PowerShell installation and
  check parsing, environment restoration and installer failure handling. Local
  Go detection disables automatic toolchain selection only for the probe.

## Verification and limits

The complete Go run passes 1,072 top-level tests and 347 subtests, with 12
platform-dependent tests skipped. Final focused regression runs also pass,
including built-desktop registration rollback. All 123 GUI unit tests and both
Chromium update interaction tests pass. The portable PowerShell subset passes
33 tests/subtests, skipping three tests that require Windows itself.

CLI and native desktop builds, `go vet`, module verification, documentation
checks, `actionlint`, GUI production build and the GUI unit tests pass on Linux.
Windows amd64 and macOS arm64 test executables cross-compile. The app upgrade
browser tests are also registered as `npm run test:update` and run on the Linux
release CI job.

The complete Go suite is run excluding only
`TestGoTestsLiveInTopLevelTestDirectory`: an unrelated local
`internal/filechanges/snapshot_test.go` violates that repository layout rule.
That file is preserved. Platform-specific tests still skip when their required
native operating system is unavailable.

Windows file locking, COM shortcuts, WebView2 and macOS LaunchServices/native
window behavior require native runner execution. Cross-compilation and portable
PowerShell do not certify those OS integrations. Likewise, OS application-menu
behavior after a live split/standalone conversion needs native validation.
The existing release CI matrix runs Go tests on Linux, Windows and macOS; it has
not been triggered remotely as part of this local audit. No claim of zero
possible failures under all networks, disk conditions or historical releases is
made. `govulncheck` reports no vulnerabilities with Go 1.27.3.
