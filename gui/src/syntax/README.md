# Cursor syntax highlighting

The editor, Markdown code blocks, and CODE source panels share Cursor Dark token
colors and font styles through Shiki's TextMate/Oniguruma engine. Semantic tokens
from language servers are deliberately excluded from this implementation.

`cursor-dark.json`, `cursor-languages.json`, and `grammars/` are snapshots from the
local Cursor installation. `SOURCES.json` records the version, original extension
paths, and declared licenses. Grammar scopes, includes, and injections are kept
intact. `cursorGrammars.ts` loads the chosen grammar and its dependencies on demand.

Languages not present in that snapshot use the Shiki catalog in
`bundledLanguages.ts`, with the same Cursor Dark theme. Both sets are bundled with
Solomon; highlighting does not require Cursor to be installed or a network
connection at runtime. Unknown file types remain plain text.

Production builds move large grammar definitions and the Oniguruma binary into
local JSON/WASM assets through `syntaxAssetsPlugin.ts`. Their existing dynamic
imports load these assets only when highlighting needs them. `gui_bundle.go`
includes the assets in both binaries; no external download is required.

The CodeMirror adapter changes only decoration ranges. Token updates leave text,
selection, autosave, and explicit Enter handling intact. Markdown uses escaped
React text nodes and copies the original source rather than highlighted markup.

Reference: https://shiki.style/guide/load-theme
