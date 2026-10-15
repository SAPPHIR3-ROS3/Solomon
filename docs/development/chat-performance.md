# Measuring thread navigation

Thread switches automatically record a bounded history of 100 samples on the
device. Open **Settings → Advanced → Chat performance**, switch between threads,
then return and choose **Refresh measurements**. **Copy report** and **Export JSON**
include every phase; the report survives reloads and hot installations in the
same browser profile. **Clear measurements** starts a fresh measurement session.
The report includes IDs and timings, but no message text, titles, project paths,
or attachment data. The GUI build timestamp appears in the panel and in each
sample, so exports distinguish measurements taken before and after a hot install.

The browser console also exposes `window.solomonPerformance.report()`, `.clear()`
and `.download()` after the first navigation or opening the diagnostics panel.

| Metric | Meaning |
| --- | --- |
| `cached` | The requested transcript was already in frontend memory |
| `selectedMs` | Time from navigation handler entry to selecting the transcript |
| `commitMs` | Time until ChatView's layout effect, including React and Markdown rendering |
| `paintOpportunityMs` | Time to the second animation frame after commit; a paint opportunity proxy |
| `chat.endpoint` | Resolving the desktop daemon URL |
| `chat.headers` | Fetch until response headers; includes browser scheduling and network/server time |
| `chat.body` / `chat.json` | Reading the response body / synchronous JSON parsing |
| `chat.normalize` / `cache.save` | Payload conversion / saving the frontend snapshot |
| `chat.total` | Entire transcript request and conversion |
| `git.branches` / `git.worktrees` | Concurrent Git metadata requests |
| `longTasks` | Browser tasks longer than 50 ms overlapping navigation through 500 ms after the paint opportunity |
| `highlighting` | Aggregate elapsed highlighting promises, including grammar loading; not pure CPU time |

Ready does **not** mean every asynchronous syntax highlight or image has finished.
Spans overlap and must not be added together. Long tasks are unavailable in some
WebViews and include other work on the same browser main thread. Hidden windows
throttle animation frames; measure while the app is visible. Counts and character
size help compare equally sized histories. Errors and cancelled samples are kept
but excluded from latency percentiles. Capture both cold first opens and repeated
switches: a fast cache selection with a slow commit points to rendering rather than
a missing prefetch.

## Reproducible benchmark

From `gui`:

```sh
npm run benchmark:chat -- --messages 20,100,300 --repeats 6 --output /tmp/chat-performance.json
```

This builds a separate, optimized production fixture, runs the actual ChatView
and useChatRuntime in headless Chromium, and reports first-load and cached median,
p95 and maximum latency. Histories contain Markdown, tables and code blocks. The
server is simulated with a declared delay (default 50 ms; change with
`--server-delay`). The first two switches populate the cache, followed by the
requested number of repeated switches. It does not contact the user's daemon or
change conversations. Temporary bundles are removed after the run. Set
`SOLOMON_TEST_BROWSER` to select a Chromium executable.

Synthetic results expose size-dependent rendering costs; they are not measurements
of the installed desktop WebView or the user's real transcripts. Use the exported
in-app report to validate the actual regression before choosing an optimization.

## Linux desktop engine and interactions

The native Wails client on Linux uses WebKitGTK, which can behave differently
from Chromium. Run the production fixture in an isolated GTK WebView under Xvfb:

```sh
node gui/scripts/benchmark-chat-navigation.mjs --engine webkitgtk --floating-letters --tools --interactions --messages 100,300,1000 --repeats 6 --max-cached-ms 750 --output /tmp/desktop-chat-performance.json
```

Requires `xvfb-run`, `/usr/bin/python3`, PyGObject and the WebKit2 4.1 GI bindings.
It uses the installed WebKitGTK engine and a desktop `ServerBridge` fixture;
it does not attach to or restart the live Wails app and never writes real chats.
The JSON report identifies the engine/version and includes cold and cached
navigation timings. `--tools` adds tool cards; `--interactions` exercises draft
restoration, tool expansion/collapse, scrolling during transcript updates,
prepending older history without moving the reading anchor, returning to the
bottom, returning from Editor, and rapid thread switches. It counts card geometry
reads after the interface settles and fails if idle decoration repeatedly
measures historical cards. Uncaught errors and rejected promises fail the run.
`--engine chromium` runs the same fixture and assertions in Chromium.

Only the recent 60 messages (extended to a complete turn) mount initially.
Earlier history appears in pages when scrolling to the start or selecting
**Load earlier messages**. The window and reading position survive
thread and Editor switches, and are stored in session storage for reloads. Parsed Markdown trees use a bounded memory cache;
no rendered DOM or component interaction state is shared between messages.

`npm run test:desktop:linux` runs the WebKitGTK interaction checks with a 750 ms
cached p95 budget. Reports are written before timing-budget failures are raised.

`npm run test:animation:desktop` measures actual canvas draw cadence in isolated
WebKitGTK at high letter density, with 100- and 300-message histories and tool
cards. Each streaming sample lasts at least five seconds. It reports idle and
streaming FPS, frame-gap p95/p99/maximum, frames exceeding 25/50 ms, draw-time p95
and range measurements per frame. It fails below 55 average FPS or above a 50 ms
maximum frame gap; the JSON report is written before a budget failure. These
numbers describe the test host, not a guarantee for every desktop/GPU. Keep this
timing run isolated from builds and other test workloads.

Flight targets are cached until content, scrolling, fonts or layout changes.
Highlights remain registered between frames and are removed as letters land;
they are still rebound synchronously when Markdown replaces text nodes.
Historical indexed entries retain identity through tail updates, including
fallback checkpoint context, so unchanged user/assistant turns can skip React
renders during token presentation. A browser regression verifies zero range
measurements and zero highlight rebuilds across unchanged in-flight frames.

`npm run test:composer` and `npm run test:composer:desktop` check the real ChatView
in Chromium and isolated WebKitGTK. Sending a follow-up must leave the preceding
answer's completed duration intact, with no live counter until the new assistant
turn begins. Composer checks cover the final character moving a complete word
to the next line, editing inside that word, deletion moving it back, explicit
newlines, long unbroken input and width changes without another keystroke. They
also reject page errors, including ResizeObserver feedback loops.

These interaction checks also cover reasoning availability: first-token timings
without reasoning text show a static "thought briefly" label with no expansion
control, including collapsed tool activity. Actual reasoning remains expandable with or without a duration and
appears when its streamed text arrives. Collapsed tool summaries aggregate only
the durations of messages containing reasoning text.

Floating-letter presentation has a separate queue: a visual token contains up to
four Unicode graphemes and advances every 60 ms. Fast network chunks and dropped
frames never reveal a full line to catch up. A finished stream drains its queue;
disabling the effect or reduced-motion preference reveals the stored text
immediately. Navigating away cancels the queue and reopening a thread does not
replay its history. Native interaction checks include burst pacing, stream
completion with queued tokens, and leaving the view during a queued response.

Presentation identifies a response by its role and checkpoint sequence. The
temporary streaming ID becomes a server ID in the final snapshot; treating that
as a new response used to reset the visual queue and replay the settled text.
The queue and canvas now share the checkpoint identity while message actions
continue using the server ID. Browser and native checks replace IDs with a real
`chat_snapshot` event during the queue and after landing, checking monotonic
progress, small token steps and zero flights for already settled letters. A
separate checkpoint containing identical text must still animate normally.

The native checks also finish an incomplete bold span after its letters have
landed and count subsequent flight paints. Closing Markdown syntax must produce
zero new flights for existing glyphs. Text reconciliation keeps each response's
rendered baseline, preserves retained glyph positions across formatting changes,
and bounds the detailed diff so long rewrites cannot create quadratic work.

Inline delimiter tests also close `**`, `*`, `__`, `_`, `~~` and backticks while
letters are still flying. A MutationObserver samples highlight bindings in the
same commit, before the next animation frame: each range must already reference
the new formatted text. This catches a one-frame reveal/hide flash that checking
only completed animations cannot detect. Rebound flights retain their original
start time and update their font without restarting their trajectory.
