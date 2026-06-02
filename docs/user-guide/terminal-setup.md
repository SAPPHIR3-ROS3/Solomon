# Terminal setup (fonts and colors)

Solomon is a terminal application: it writes text and ANSI styling to stdout. Your **terminal emulator** chooses the font, size, and ligatures — Solomon cannot change those from code or from `config.toml`.

## Font (required)

Use a **monospace** font in your terminal settings. Proportional fonts break alignment of the welcome banner, tool output, and readline prompt.

**Turn off ligatures** in the terminal font settings (or pick a “Mono” variant without ligatures). Ligatures merge character sequences (e.g. `=>`, `!=`) into single glyphs and can make copied text and layout misleading in a CLI.

### Suggested fonts by platform

| Platform | Examples |
| -------- | -------- |
| Windows | Cascadia Mono, Consolas, JetBrains Mono |
| macOS | SF Mono, JetBrains Mono, Menlo |
| Linux | JetBrains Mono, Fira Code Mono, DejaVu Sans Mono, Ubuntu Mono |

Solomon does not verify or enforce a font name at runtime.

## Emoji and symbols

Assistant and tool output may include emoji or Unicode symbols. Solomon does not adjust grapheme width or alignment for them. Minor visual misalignment is expected and does not affect chat content or persistence.

## Colors

Styling is handled by [`internal/termcolor`](../../internal/termcolor/) on top of [lipgloss](https://github.com/charmbracelet/lipgloss) and [termenv](https://github.com/muesli/termenv). The palette is fixed for **dark** backgrounds; there is no light-theme mode.

### When output is plain (no ANSI colors)

Colors are disabled when any of the following applies:

| Condition | Effect |
| --------- | ------ |
| stdout is **not** a TTY (pipe, redirect, capture) | Plain text only |
| `NO_COLOR` is set to any value | Plain |
| `CLICOLOR=0` | Plain |
| `solomon exec --no-color …` | Plain (also sets `NO_COLOR` for child tools) |

Piping overrides `FORCE_COLOR`: redirected stdout never receives color codes from Solomon.

Interactive REPL on a real terminal uses colors when none of the disable rules above apply.

### Machine-readable runs

With `solomon exec --json` or `--jsonl`, stdout is JSON. Use a pipe or `--no-color` if you need to guarantee no escape sequences on stdout. Diagnostics go to stderr.

### Logo and readline

The Braille welcome logo uses the same color policy as the rest of the UI (including downgrade on limited terminals). On Windows, `[img-n]` tags in the readline buffer use a reduced ANSI palette so readline’s translator does not choke on truecolor background sequences.

## Tab completion (interactive REPL)

While the REPL prompt is active, Solomon runs its own line editor in **raw mode**. The **Tab** key is handled inside Solomon (not by bash, zsh, or PowerShell tab completion on the host shell). This applies in Terminal.app, Ghostty, GNOME Terminal, Konsole, Windows Terminal, and other VT-style emulators.

| Key / action | Behavior |
| ------------ | -------- |
| **Tab** | Complete `/` slash names and skill tokens; slash arguments (e.g. `/reasoning`, `/log`, `/add`, `/remove`, `/resume`, `/goto`); on shell lines (`!…` or shell-first): **PATH binaries** on command tokens (including after `\|`, `\|\|`, `&&`, `;`), **`go` subcommands** (from `go help`) as the second token after `go`, and **workspace paths** on file-like tokens. |
| **Tab** again | Cycle or list candidates when more than one match remains. |
| **Ctrl+C** | Cancel an open completion menu (same as canceling other readline modes). |

Set `SOLOMON_NO_COMPLETE=1` to disable REPL tab completion (Tab then does nothing useful beyond readline’s default bell).

On **Windows**, prefer **Windows Terminal** (`WT_SESSION`) for reliable input; legacy `conhost` may still work but Quick Edit and mouse selection can interfere with the line editor.

### Manual QA checklist (after changes to completion)

1. `/mo` + Tab → completes toward `/models`.
2. Double Tab on a partial `/` command shows a candidate list.
3. `/reasoning l` + Tab → completes toward `low`.
4. `!go te` + Tab → completes toward `test`.
5. `!g` + Tab → offers PATH matches (e.g. `go`, `git` when on PATH).
6. Buffer containing `[img-1]` + Tab does not corrupt the tag display.
7. Smoke-test on at least: macOS Terminal or Ghostty, one Linux terminal, Windows Terminal.

## See also

- [Usage and commands](usage-and-commands.md) — `--no-color` and exec flags
- [Supporting packages](../architecture/supporting-packages.md) — `termcolor` implementation
- [Runtime and REPL](../architecture/runtime-and-repl.md)
