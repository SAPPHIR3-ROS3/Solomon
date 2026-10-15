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
