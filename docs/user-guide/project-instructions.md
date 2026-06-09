# Project instructions and custom rules

Solomon loads agent instruction files and custom rules into the **system prompt** on every assistant turn. This matches the cross-tool `AGENTS.md` convention used by Codex, Cursor, and others.

## Rules vs instruction files — what goes where

Solomon treats these as two different layers. Use the right one so the model gets **architecture** in markdown files and **habits** in numbered rules.

| Layer | Purpose | Examples |
|-------|---------|----------|
| **Custom rules** (`/add rule`, `/add projectrule`) | Small preferences, tone, and corrections — minor things that are nice when followed consistently | Writing style; comment style; “don’t use emoji in commit messages”; “prefer `errors.Is` over string comparison”; “answer in Italian unless asked otherwise” |
| **Instruction files** (`AGENTS.md` and fallbacks) | Big, architectural context that shapes *how the software is built* | Tech stack and versions; programming philosophy; module boundaries; patterns to follow or avoid; links or references to other projects/repos as design references; deployment or testing strategy at project level |

**Custom rules** are one-liners (or very short notes) for polish and recurring annoyances. They should not replace a proper `AGENTS.md` when you need to explain stack choices, architecture, or project-specific conventions that affect structure and design.

**Instruction files** are markdown documents you edit in the repo or under `~/.solomon/`. They carry the “why” and “what” of the codebase — the kind of context that would belong in a technical onboarding doc, not in a quick reminder.

Both are injected into the system prompt, in separate sections: **Custom rules** first, then **Global instructions**, then **Repository instructions**.

## Instruction files

### Priority per directory

In each directory Solomon looks for, in order:

1. `AGENTS.md`
2. `CLAUDE.md` (only if `AGENTS.md` is missing)
3. `GEMINI.md` (only if both above are missing)

Use these files for **project-level and architectural** context: languages and frameworks, how layers fit together, conventions that affect design, pointers to other codebases or docs as reference, testing/build/deploy expectations, and similar “shape of the software” material.

### What is loaded when

| Source | Path | When |
|--------|------|------|
| Global | `~/.solomon/AGENTS.md` | Always (every chat) |
| Repository root | `<project>/AGENTS.md` (or fallback) | Always |
| Subdirectories | `<project>/<subdir>/AGENTS.md` | Only after you **work in that subtree** during the session |

Subdirectory files are **not** scanned upfront. They enter the prompt when a tool touches a path under that folder:

- `readFile` / `editFile` — from the resolved file path, Solomon walks up to the repo root and activates any `AGENTS.md` (or fallback) found on the way. Use `editFile` with `delete: true` to remove a file (do not use `shell` for file deletion).
- `shell` — path-like tokens in the command (for example `./src/lib/...`) trigger the same activation.

Once activated for a chat session, a subdirectory stays in the prompt for later turns (stored in the session file).

### Truncation

Each instruction file is capped at **32 KB** in the system prompt. Larger files are truncated with a footer noting how many bytes were omitted. Edit the file on disk to reduce size.

### Subagents

Nested agents inherit the same custom rules and instruction files as the parent session. If a subagent uses a custom system prompt file (`sysPromptPath`), Solomon **appends** that file after the inherited instruction block — it does not replace it.

When `[tools].legacy` is enabled and the custom file is not the full build template, Solomon also appends the legacy tool-invocation syntax to that nested system prompt. With `[tools].legacy_force`, it also appends the build-mode tool dump (API tool schemas are not sent in force mode).

## Custom rules

Short rules (single phrases or sentences) for **minor but recurring preferences**. They live outside markdown instruction files and are managed with slash commands.

Good fits for rules:

- Comment and prose style (e.g. “comments in English”, “keep docstrings one line when obvious”)
- Small behavioral nudges (e.g. “don’t refactor unrelated code”, “ask before adding dependencies”)
- Personal or team habits that are not architectural (e.g. naming taste, formatting quirks Solomon should respect)

Avoid putting in rules what belongs in `AGENTS.md`: stack choices, folder layout philosophy, integration with external systems, or references to whole projects as architectural templates — use instruction files for that.

| Scope | Storage | Command |
|-------|---------|---------|
| Global | `~/.solomon/rules/rule_NN.txt` | `/add rule <phrase>` |
| Project | `~/.solomon/projects/<project-id>/rules/rule_NN.txt` | `/add projectrule <phrase>` |

- `/rules` — list global and project rules with numbers and previews.
- `/remove rule <N>` — delete a global rule and **renumber** remaining files (`3` and `03` both work).
- `/remove projectrule <N>` — same for project scope.

Global and project rules appear together in one **Custom rules** section in the system prompt (global first, then project).

Custom rules and instruction files may be written in any language. `/language` sets the assistant reply language (`response_language`); the model follows rule and instruction intent regardless, but natural-language assistant output stays in the configured language.

Put **architecture and stack** in `AGENTS.md` (or repo/subdir instruction files); use rules only for the small stuff you want the assistant to remember every turn.

## Slash commands

| Command | Role |
|---------|------|
| `/add rule <phrase>` | Append a global custom rule (`~/.solomon/rules/rule_NN.txt`) |
| `/add projectrule <phrase>` | Append a project-scoped custom rule |
| `/remove rule <N>` | Remove global rule `N` and renumber the rest (`3` and `03` both work) |
| `/remove projectrule <N>` | Same for project rules |
| `/rules` | List global and project rules with previews |
| `/instructions` | Print global `~/.solomon/AGENTS.md` (path, size, body) |

Repository and subdirectory `AGENTS.md` files are edited and read on disk; only the global file is shown by `/instructions`.

## Limits and current behaviour

**What activates subdirectory instructions**

- `readFile` / `editFile` on a path under a subtree
- `shell` when the command contains path-like tokens (for example `./packages/foo/...`)

Mentioning a path in a user message **without** a tool touching that path does **not** activate subdirectory instructions.

**Visibility**

Activation is silent: there is no REPL message when a subdirectory `AGENTS.md` enters the prompt. To see which subtrees are active in the current chat, open the session JSON (`activated_instruction_dirs`) or infer from recent tool paths.

**Size limits**

- Each instruction **file** is capped at **32 KB** in the system prompt (truncated with a footer if larger).
- Custom **rules** have no separate per-rule cap; keep them short. There is no global budget across all rules — avoid dozens of long rules.

**Reload**

Instruction files are re-read when their modification time changes (no `/reload` command). Edit `AGENTS.md` on disk and the next assistant turn picks up changes.

**Not loaded**

- `.cursor/rules` and other IDE-specific rule formats
- Subdirectory `AGENTS.md` files until a tool activates that subtree in the session

## Environment override

For tests or alternate layouts, set `SOLOMON_HOME` to replace `~/.solomon` as the Solomon data directory (same effect as the default home path for global agents and rules).

## See also

- [usage-and-commands.md](usage-and-commands.md) — full slash command list
- [data-layout.md](data-layout.md) — on-disk layout under `~/.solomon`
