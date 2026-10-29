# CloakBrowser automation

Solomon exposes its managed headless CloakBrowser through deferred `cloakBrowser` and `sdk.CloakBrowser(tool, args, intent)`. Discover current schemas through `searchTools`. This native backend does not access personal browser profiles and requires no packages beyond the existing `cloakbrowser` and `playwright-core`.

## Capabilities

The catalog contains 41 operations: ten interaction, six navigation, viewport resize, structured snapshot, two console, CSS inspection, two network, three performance, fourteen heap operations and virtual WebAuthn. Screenshots, screencasts, arbitrary JavaScript, Lighthouse, extensions, PWA and site-provided tools are not included.

In the desktop app, each project/chat pair owns an isolated browser client across consecutive message turns. A turn runtime borrows that client instead of closing it after its response. Deleting the chat or shutting down the daemon closes its browser; daemon restarts and interrupted browser processes do not preserve live tabs. CLI runtimes continue to own and close their own browser clients.

Create a tab with `new_page` and retain its string `pageId` for page-scoped calls. All calls require a nonempty intent. Startup is lazy. Existing search/fetch adapters retain their interface and HTML processing in Go.

## Interaction

`take_snapshot` returns visible DOM elements and snapshot-scoped uids, not a full accessibility tree. Use the latest snapshot for interaction and CSS. Snapshot uids are tied to the captured document and original nodes: pushState, replaceState and hash changes preserve them, while reloads, node removal/replacement and a newer snapshot invalidate them. Actions pin the original element rather than retrying against a replacement. Main-document elements are supported; cross-origin frames and closed shadow roots are not covered. Snapshot attributes are observable by page scripts. Password field values are omitted, but other content may contain secrets.

Dialogs return `pendingDialog`; call `handle_dialog` before continuing. Uploads read local files and must respect the workspace boundary. Navigation accepts HTTP, HTTPS and `about:blank`; no initialization scripts are exposed. Headless pages have no desktop focus behavior. Resize changes the viewport. Named `isolatedContext` values create separate cookie/storage contexts.

## Diagnostics

Network/console capture starts at page creation with bounded buffers. Sensitive headers such as cookies and authorization are redacted; URLs, bodies, console output and heap contents may still contain secrets. Body reading is size/encoding limited. Diagnostic filesystem paths are unsupported.

Performance uses bounded native CDP traces. Tracing is browser-wide. Supported insights are listed in results and are not equivalent to DevTools frontend insights; unsupported names fail explicitly. No CrUX service is used.

Heap snapshots use in-memory `snapshotId` values and count/byte limits; release them with `close_heapsnapshot`. Comparison uses `baseSnapshotId` and `currentSnapshotId` from the same page lifetime. Classes use numeric `id` values from details; objects use numeric `nodeId`. Dominators model non-weak edges, not every V8 ephemeron/ownership heuristic. Context analysis reports structural references, not dead captured fields or guaranteed reclaimable memory.

CSS returns bounded raw CDP rules, inherited and computed styles, without DevTools cascade winner labels. WebAuthn creates virtual test authenticators, not real passkeys, and cannot bypass authentication.

## Verification

Tests live under `test/`. Run the installed browser against a local HTTP fixture with `SOLOMON_TEST_CLOAK=1 go test ./test -run '^TestCloakCapabilitiesRealBrowser$' -count=1`. No external websites are required. Diagnostic unit tests run with `node --test test/cloak/*.mjs`.
