# Programming Languages

Open Settings → Programming Languages, immediately below Network. Solomon records
languages when source files are opened or code is displayed. Restored editor tabs
also contribute their languages. The history is shared by browser and desktop
clients using the same Solomon host and is stored in `config.toml` under
`[programming_languages]`. It does not scan the computer or reconstruct files that
were closed before this feature existed.

The editor status bar shows only the language name, without icons, immediately after
UTF-8. Click it to choose Automatic, any detected installed server, or Internal
from the same upward-opening menu style as the branch selector. Detailed fallback
reasons remain in the label tooltip. The selection is saved per language.

Each language offers three assistance modes:

- **Automatic** looks for known, already installed language-server commands on the
  Solomon host's PATH. This includes SDK-provided commands such as Dart's
  `dart language-server`, Gleam's `gleam lsp`, and Nushell's `nu --lsp`, as well as
  separately installed servers such as `gopls` and `pyright-langserver`.
- **Custom language server** accepts an executable and its arguments as a JSON
  array. The process runs directly in the registered project's directory, without
  an implicit shell. Use a complete executable path when it is not on PATH.
- **Internal fallback only** keeps built-in editing and syntax highlighting and
  starts no language server.

Solomon never downloads a server or installs a language dependency automatically.
An installed compiler or runtime does not necessarily include a language server.
Detection checks executable availability; the editor displays LSP as connected
only after initialization succeeds. A missing executable, failed initialization,
or disconnected server leaves the document editable with the internal fallback.
Remote clients use language servers installed on the machine hosting Solomon.

The integration currently supports document synchronization, server diagnostics,
completion and hover information. Completion, hover and diagnostics can each be
disabled per language. It does not advertise support for server-initiated workspace
edits. Go-to-definition, project-wide rename and other refactorings are not exposed
by this integration yet. Internal highlighting remains enabled in every mode.

Server settings and initialization options are JSON objects. Settings are sent in
`workspace/didChangeConfiguration` and served through `workspace/configuration`.
Initialization options are passed to `initialize`. Their supported keys depend on
the selected server. Saving settings reconnects the open editor so changes apply.
The editor closes its server connection when its document view is destroyed.

Example configuration:

```toml
[programming_languages.python]
name = "Python"
first_used = "2026-11-10T12:00:00Z"
last_used = "2026-11-10T12:00:00Z"
mode = "custom"
command = ["pyright-langserver", "--stdio"]
completion = true
hover = true
diagnostics = true

[programming_languages.python.settings.python.analysis]
typeCheckingMode = "strict"
```

## Language preferences independent of LSP

These preferences can be implemented and changed without a language server; this
page currently edits LSP preferences rather than adding all of these controls:

| Preference | What supplies it |
| --- | --- |
| Tabs versus spaces and indentation width | Editor and optional project configuration |
| Indentation guides and their click behavior | Editor |
| Line wrapping, line numbers and whitespace display | Editor |
| Comment and uncomment commands | Language's comment delimiters |
| Automatic parentheses, quotes and indentation | Editor and language syntax rules |
| File extensions, special filenames and shebang associations | Language catalog |
| Syntax highlighting and bracket colors | Internal grammar and theme |
| Encoding and LF/CRLF line endings | File loading and saving |
| Formatting | Either a standalone formatter or an LSP formatting provider |
| Build, run and test commands | Language toolchain or project tasks |

The formatting and project-task rows describe independent integration options,
not features newly implemented by this settings page.
