# Native WebKit inspector

On Linux, `make desktop-inspector` builds
`gui/desktop/build/bin/solomon-inspector` with the existing Wails debug and
devtools build tags. It does not replace the installed application. Local GTK
and WebKit development libraries are resolved by the existing Makefile settings.

Run `./scripts/inspector.sh` to open a separate Solomon window with the native
WebKit Inspector. Ctrl+Shift+F12 reopens the Inspector. The window connects to the
existing daemon and real frontend; it shares real conversations and settings.
Do not submit messages or change settings during read-only diagnosis. Closing
this diagnostic window does not close the original application.

Use the Timelines recording to capture rendering frames and JavaScript activity
while the flicker occurs, together with a screen recording. Keep letters,
density, blur, window size and engine unchanged between comparisons. An open
Inspector can affect timing; also reproduce with it detached or closed.

`SOLOMON_WEB_INSPECTOR=1` opens the Inspector at startup only in a debug build.
Normal production builds keep devtools disabled. This does not expose a remote
inspection port.

## Baselines prepared on this machine

Commits are numbered from the oldest reachable commit in `git log --reverse main`.

- 800: `92371fe2919ee8e8a0b998fbeeaa8b5200b622c4`
- 790: `0027e033d198618f13e7789cc008ed977c313b9f`

GUI source snapshots are under
`/home/oni/.local/share/solomon/inspector/baselines/{800,790}/gui`.
They were exported with `git archive`; no checkout, branch or worktree was changed.
They are source snapshots, not yet runtime-validated baseline applications.
The native inspector build uses the current working tree, including local edits.

The original recording is untouched. The reduced copy is
`/home/oni/Videos/Screencasts/Solomon-flicker-last-26s.mp4`:
source interval 60 seconds through the end, 25.785 seconds, 1280 × 950,
29 fps, 4,401,457 bytes. It contains video only, encoded using installed
GStreamer components. Compression can obscure subtle pixel changes; use the
original for frame-level confirmation.

## Automated recording

`gui/scripts/inspect-rendering.mjs` builds a private production fixture and records
idle/streaming frame samples in isolated WebKitGTK. It accepts `--source-root`,
`--response` (a JSON string file), `--offset`, `--duration-ms` and `--output`.
`--original-blur` enables the original composer blur inside that fixture.
`--native` uses the dedicated inspector window instead of Xvfb.

To start that dedicated native window without opening the local Inspector UI:

```sh
SOLOMON_WEB_INSPECTOR=0 WEBKIT_INSPECTOR_SERVER=127.0.0.1:39221 gui/desktop/build/bin/solomon-inspector
```

The native recorder uses the existing system Python GLib bindings and WebKit's
loopback inspector transport. No new dependency is required. It records native
Timeline events, frame geometry, errors, and a final page snapshot.

```sh
node gui/scripts/inspect-rendering.mjs --native --original-blur --duration-ms 20000 --output /tmp/rendering.json
```

For a complete-app recording, open a fresh dedicated inspector window and run:

```sh
/usr/bin/python3 gui/scripts/inspector.py --live-title '<visible chat title>' --original-blur --duration-ms 30000 --output /tmp/live.json
```

Adding `--chat-id '<chat ID>' --replay-offset 2400` replays the first nonempty
assistant response in the diagnostic window's frontend memory. It restores that
snapshot after a successful capture and never submits or saves a message. Use
only the dedicated diagnostic window for replay. The tool stops rather than
recording if the expected page or floating-letter canvas is absent.

The verification report from this investigation is stored at
`/home/oni/.local/share/solomon/inspector/verification.md`. A reduced clip covering
the reported onset is
`/home/oni/Videos/Screencasts/Solomon-flicker-evidence-35-60s.mp4`.
