# Solomon terminal explorations

Four standalone Bubble Tea mockups. No daemon connection, model calls, credential access or execution of archived commands. The real chat is replayed as data; additional gallery states and new replies are simulated.

## Run

From this directory run `./atelier`, `./orbit`, `./paper` or `./mission`. The Linux amd64 executables are ignored by Git. Add `--demo` for the complete archived conversation, `--artifacts` for the illustrative gallery, or `--image` for the real embedded image. All fixtures and the PNG are embedded in each binary.

| Prototype | Direction |
| --- | --- |
| Atelier | Original Solomon banner and classic artifact rails |
| Orbit | Centered welcome group and rounded artifact rails |
| Paper | Compact left-aligned welcome and minimal artifact rails |
| Mission | GUI-derived welcome: free crown with greeting overlay, workspace row, rounded composer with internal toolbar, branch controls and stats |

The original crown, colors, split-banner junctions and gold framing remain Solomon's shared identity. Orbit centers the classic framed banner/composer group. Mission reproduces the structural layout of `gui/src/home/Welcome.tsx`, `gui/src/chat/ChatComposer.tsx` and `gui/src/home/welcome.css`: the original crown is unframed, the greeting overlays its middle, the workspace sits immediately above the rounded composer, model/reasoning/Agent/send controls sit inside it, and branch/worktree controls and stats sit below. The complete stage is centered and width-constrained like the GUI. After the first message the composer docks at the bottom. User messages have complete gold frames. Chat, normal artifacts and composer use the terminal's native background. Mockup-only controls use Solomon blue (#061c3b): layout/comparison toolbar, command-center comparison panel, fixture provenance, mockup slash commands and archive links/counters. Image pixels retain their own real colors. The redundant mode label under the composer and visible artifact/row counters have been removed.

## Controls

| Key | Action |
| --- | --- |
| F1–F4 | Switch layout, preserving draft and reading anchor |
| F7 or /components | Live component map |
| F5 / F6 | Open archived conversation / illustrative gallery |
| Enter | Execute a highlighted slash command or submit a simulated prompt |
| Ctrl+J / Alt+Enter | Insert a newline |
| Shift+Enter | Newline when encoded as CSI-u or modifyOtherKeys by the terminal |
| / | Open filtered command suggestions |
| Up / Down in suggestions | Select a slash command |
| Tab in suggestions | Complete the selected command |
| Escape in suggestions | Dismiss suggestions |
| Up / Down otherwise | Recall prompt history |
| Tab otherwise | Toggle Agent / Chat preview mode |
| Ctrl+U | Clear draft |
| Mouse wheel / PgUp / PgDown | Scroll the whole transcript |
| Home / End | Transcript start / end |
| Alt+Up / Alt+Down | Previous / next artifact |
| Ctrl+O | Fold or expand the artifact at the viewport's top |
| Ctrl+D | Fold or expand all artifact bodies |
| Ctrl+N | Start a new thread |
| Ctrl+K | Command center |
| Ctrl+P | Model selector (preview-only choices) |
| Ctrl+R | Reasoning selector (preview-only choices) |
| Ctrl+Q / Ctrl+C | Exit and restore terminal attributes |

Slash suggestions list commands actually implemented in this mockup: `/agent`, `/chat`, `/new`, `/clear`, `/demo`, `/artifacts`, `/image` and `/help`. Slash commands execute even during simulated streaming. `/image` replaces the active response with the real PNG view immediately. Preview selectors and mode labels do not change live Solomon settings.

## Presentation

Tool cards follow the desktop's semantic structure: intent/status, tool name, readable parameters and result. `searchTools` displays its query and matched tool names, descriptions and SDK signatures, not argument/result JSON. `orchestrate` displays full Go source with line numbers and token-level syntax colors, followed by its full result.

The Go lexer uses the standard library scanner and preserves source text. Markdown supports headings, inline bold/code, lists, links, tables and fenced code; it is a terminal subset, not a complete browser/CommonMark renderer. Long content wraps and scrolls. Folding is reversible; it never removes underlying source or output.

`/image` decodes the original workspace `icon.png`, copied byte-for-byte into `fixtures/preview.png`. The terminal displays its actual pixels using truecolor half-block cells, preserving aspect ratio and transparency where possible. This is a real raster preview, not a text placeholder or a native Kitty/iTerm image protocol. No external codec or new dependency is used.

## Archive provenance

Reference chat: `03fcff4b9b2fc7e1625ea382b21fb1ea002a4aa496151239de749889659deb01`, `rendere-le-immagini-visibili-nell’editor`, 2026-10-25. `fixtures/editor-images.json` retains the complete saved content, grouped into 38 presentation blocks. Tool calls and results are paired. No source/output excerpt markers or character/line limits are added by the mockup. If the historical tool itself truncated an output, that original recorded result remains unchanged. Git/CI commands in the archive are displayed, never executed.

The separate artifact gallery contains illustrative plan, decision summary, diff, table, subagent, permission, error, shell, /btw, checkpoint, notification and usage states. These are not claimed as events from the real chat. The gallery image is an actual embedded asset.

## Input and scrolling

A stateful input normalizer handles SGR and legacy X10 wheel/click reports before Bubble Tea decoding, including fragmented reads. Mouse bytes never become composer text. Bracketed paste is preserved. A normalized pipe avoids races between terminal polling and buffered reads. Raw terminal attributes are restored on exit.

Scrolling clamps to real viewport limits. Layout changes, resize and fold/expand preserve an artifact anchor. Idle mockups do not repaint on a timer. The composer supports append, backspace, paste, multiline entry, history and slash completion. Cursor editing, live attachment upload, real permissions/checkpoint actions and persistence remain outside the mockup.

## Build and verify

Use `make build`, `make test`, and `make vet`. Full tests require Go and Python 3; Python uses only its standard library. Bubble Tea v1.3.10 and Lip Gloss v1.1.0 remain pinned in this isolated module. No new dependencies were introduced. The parent module is unchanged.

Tests cover 200 layout/state/size snapshots, original banner rows, frame geometry, pinned composer, semantic tool cards, multiple token colors on one Go line, actual PNG pixel cells, complete archive content, slash suggestions and Linux PTY interaction. PTY tests include SGR/X10 wheel reports, fragmented mouse sequences, Unicode, slash filtering/completion, images, resize, paging, both scroll limits, layout switches, exit and terminal restoration. This does not establish browser-level visual parity, absence of flicker, or compatibility with macOS, Windows or PowerShell. Minimum terminal: 42 × 24; recommended: 120 × 40 with truecolor.

## Mission GUI welcome

Run `./mission` with no flags to see the GUI-derived empty-thread layout, rather than `--demo` which opens a conversation. Set `SOLOMON_PREVIEW_NAME` to customize the greeting; its default is `User`, matching the GUI fallback. The existing terminal background stays unchanged. Font size, shadow and sub-cell CSS spacing cannot be copied literally into character cells, but element order, overlay, alignment, width constraint, internal toolbar and docking transition are reproduced. On smaller terminals the crown compresses only while a picker or slash menu is open.

Ctrl+P opens the model picker, Ctrl+R opens the reasoning picker, Up/Down changes selection and Enter or Escape closes it. Tab toggles Agent/Chat. Choices are local fixture values, not live model configuration. The bottom controls and token counters are mock presentation values. The other three prototypes retain their existing classic welcome layout for comparison.

Additional regressions exercise plain `/image` during an active response on all four executables, Mission control selection, greeting overlay, workspace/composer alignment and the toolbar's position inside the composer.

Scrolling PTY tests use an explicitly enabled, non-visible OSC trace (`SOLOMON_TUI_SCROLL_TRACE=1`) rather than requiring diagnostic counters in the normal interface. Normal launches emit no scroll trace and show no artifact counters.

## Live development workflow

From the repository root, `make tui-dev` opens Mission with live configuration and automatic Go hot restart. No separate daemon restart or manual frontend launch is required.

Use `make dev` and keep the terminal open. The component map shares the real renderers. Edits to `~/.solomon/customization/tui/interface.json` apply live, including shortcuts, without compiling; Go edits trigger a background candidate build and state-preserving handoff. `make components` opens the same map without source supervision. `--config <path>` or `SOLOMON_TUI_CONFIG` selects another interface file. F7 and `/components` open the map. Use `make test-live` and `make test-dev` for focused reload checks.

One `laboratory` binary now implements all four layouts. The existing variant commands are aliases (or copies where symlinks are unavailable), so `make build` no longer compiles the full frontend four times. `make dev` does not run the test suite on every edit.

The shared runtime lives in the permanent `tui/` module, not inside prototype fixtures. See `../tui/README.md` and `../docs/user-guide/tui-customization.md`. The readline frontend and daemon are not restarted or migrated by these commands.

## Mouse interaction

Mouse clicks are enabled by default. In the Bubble Tea frontend, left click activates model/reasoning selectors, Agent/Chat, send/stop, slash suggestions, layout controls, component selection and artifact header folding. Press and release do not execute an action twice. Areas are measured from the final rendered screen, including margins, Unicode widths, scrolling and resize. Decorative workspace/branch fixture labels are not presented as live repository actions.

Set `mouse` to `false` in the live interface JSON to disable terminal mouse reporting without rebuilding. This restores native terminal mouse handling; a Shift-drag selection may also be available depending on the terminal. Clicking inside composer text does not yet reposition an editing cursor. The normalizer handles SGR and legacy X10 click reports, including fragmented input, without inserting mouse bytes into the draft. The reusable `tui/interaction` module provides hit regions independently of Bubble Tea.

## Input responsiveness

Clicks, keys and wheel events do not wait for simulated responses to finish. The component map caches its formatted content across unchanged UI events, and the transcript caches individual unchanged artifacts during streaming. Configuration, component state, width and layout changes invalidate the relevant cache; ordinary scrolling only changes the visible slice. Mouse release reports that have no bound action are not replayed as extra renders. Keyboard text and click presses remain ordered and are not discarded.

The responsiveness regression sends a burst of 24 wheel events followed by a click and typed input, and separately types/clicks during a long transcript stream. It checks UI latency without changing prompt submission, cancellation or agent-message queue behavior.
