# Solomon TUI customization runtime

Permanent, frontend-independent infrastructure for TUI personalization. This local module uses only the Go standard library and is not tied to prototype fixtures, providers, credentials, or the agent harness. The Bubble Tea laboratory currently consumes it through a local module replacement. The existing readline frontend is unchanged until the TUI 2.0 migration.

## Packages

| Package | Responsibility |
| --- | --- |
| `live` | Versioned configuration, validation, context keymaps and content-based polling |
| `components` | Stable component IDs, labels and state catalog |
| `state` | Versioned UI-state envelope and atomic private checkpoint files |
| `dev` | Optional Go build supervisor, terminal handoff and candidate rollback |

## Runtime configuration

The default path is `$SOLOMON_HOME/customization/tui/interface.json`; when `SOLOMON_HOME` is unset, it is `~/.solomon/customization/tui/interface.json`. `SOLOMON_TUI_CONFIG` explicitly overrides that path. No workspace file is automatically executed or trusted.

`live.Export` creates a default file without overwriting existing content. `live.NewWatcher(path).Poll()` detects content changes and returns a complete validated configuration. An invalid document leaves `Current` unchanged and reports an error once for that content. Deleting the file restores defaults. The frontend must apply a changed configuration in its own event loop, never from the watcher goroutine.

Configuration exposes RGB color roles, composer/home dimensions, border style, text roles and context-specific keys mapped to named actions. Partial objects inherit defaults. Unknown fields/actions, unsupported versions, malformed colors, unsafe control characters, invalid dimensions and conflicting keys are rejected. Keys can be unbound with an empty array. Ctrl+C always remains an emergency exit; F17/F18 are reserved internal wheel transport. Ctrl+M and Ctrl+I normalize to Enter and Tab so conflicts are detected correctly. Shift+Enter normalizes to the supported extended-terminal newline transport; old terminals can still send the same bytes as Enter.

The module never changes model/provider credentials or harness configuration. Configuration is declarative, not executable. Files created by the tools use private permissions.

## Commands

From this module, `go run ./cmd/tui-config` creates the default interface file; `--check` validates it and `--print` emits the complete default document without writing.

`go run ./cmd/tui-dev --directory <frontend-directory> --watch <shared-source-directory> -- <frontend-arguments>` supervises a Go frontend that implements the state protocol below. Source compilation occurs only in this explicitly requested development mode.

## State and hot restart protocol

The supervisor provides `SOLOMON_TUI_STATE_PATH`, `SOLOMON_TUI_RELOAD_REQUEST` and `SOLOMON_TUI_DEV_STATUS` to its child. It builds candidates in a private temporary directory without stopping the active frontend or writing build logs over its terminal. Status JSON contains bounded compiler diagnostics.

After a successful build, the supervisor writes a random request token. The frontend reads it in its event loop, writes its presentation state with `state.Save(path, token, payload)`, verifies that the request remains current, and exits normally after restoring terminal attributes. Only then does the supervisor verify the envelope/token and start the candidate. A failed build keeps the existing frontend; a handoff timeout does not kill it. An immediately failing candidate is rolled back to the previous binary.

The envelope is versioned and limited to 16 MiB. The payload belongs to the frontend. Store UI state such as draft, active chat ID, scroll anchor and expanded cards, not authentication credentials or running harness objects. The laboratory includes its static messages and simulated streaming state. Production frontends must reconnect to daemon-owned sessions; this mechanism does not serialize or restart a live LLM request.

This is true live reload for declarative configuration and state-preserving hot restart for Go logic, not dynamic Go-code replacement. Handoff timeout is 3 seconds and immediate-failure probation is 750 ms. Filesystems are polled without external watcher dependencies.

## Verification

`make test` and `make vet` verify the module. Development supervisor tests cover compiler failure, watch filtering, invalid tokens, timeout, coalescing, cancellation, rollback and user quit. The laboratory adds Linux PTY tests for real compilation, checkpoint handoff, state restoration and terminal cleanup. Windows/macOS terminal parity remains a separate migration verification.

## Mouse interaction

Mouse clicks are enabled by default. In the Bubble Tea frontend, left click activates model/reasoning selectors, Agent/Chat, send/stop, slash suggestions, layout controls, component selection and artifact header folding. Press and release do not execute an action twice. Areas are measured from the final rendered screen, including margins, Unicode widths, scrolling and resize. Decorative workspace/branch fixture labels are not presented as live repository actions.

Set `mouse` to `false` in the live interface JSON to disable terminal mouse reporting without rebuilding. This restores native terminal mouse handling; a Shift-drag selection may also be available depending on the terminal. Clicking inside composer text does not yet reposition an editing cursor. The normalizer handles SGR and legacy X10 click reports, including fragmented input, without inserting mouse bytes into the draft. The reusable `tui/interaction` module provides hit regions independently of Bubble Tea.
