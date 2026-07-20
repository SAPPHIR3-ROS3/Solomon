# Solomon UI Lab

Standalone React prototypes for comparing the initial Solomon Agent/Editor experience. This directory does not call or import the Solomon Go backend.

## Run

```bash
npm install
npm run dev
```

Run the commands from `ui-prototypes/`. The dev server uses `http://127.0.0.1:4173`.

Direct prototype URLs:

- `http://127.0.0.1:4173/v1` — Current
- `http://127.0.0.1:4173/v2` — Atlas
- `http://127.0.0.1:4173/v3` — Pulse
- `http://127.0.0.1:4173/v4` — Quiet
- `http://127.0.0.1:4173/v5` — Deep, interactive mock chat

You can switch prototypes by changing only the final number in the URL.

## Build

```bash
npm run build
```

## Mock configuration

The gallery reads `public/mock-config.toml` at startup. It controls the active workspace, session, model settings, open files, UI layers, conversations, and per-turn changes.

## Directions

1. **Current** — pragmatic T3/Zed reference with thread, project editor, and live turn status.
2. **Atlas** — conversations and files become an operational map instead of a conventional sidebar.
3. **Pulse** — the active agent turn is the workspace clock and primary navigation structure.
4. **Quiet** — a nearly chromeless, command-led surface focused on the current conversation or buffer.
5. **Deep** — a dark, minimal chat whose composer moves from the center to a bottom dock after the first message. Its model picker reads the current and recent models from Solomon Home, while mock responses stream one Lorem Ipsum word for every word in the user prompt.

The directions share data and capabilities, not a common dashboard layout. Chat selection, file opening, Agent/Editor state, turn changes, and CodeMirror editing are interactive.
