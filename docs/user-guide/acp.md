# Agent Client Protocol

Run `solomon acp` to serve stable ACP v1 over stdio. Messages use newline-delimited JSON-RPC 2.0; stdout is reserved for the protocol and diagnostics go to stderr. No new dependency is needed. Configure T3 Code with executable `solomon` and arguments `["acp"]`.

## Supported agent methods

| Method | Behavior |
| --- | --- |
| `initialize` | Negotiate v1 and advertise implemented capabilities |
| `session/new` | Initialize a provider and an independent conversation at the requested absolute working directory |
| `session/prompt` | Process text, images, resource links, and embedded resources; stream messages, thoughts, tool calls and results |
| `session/cancel` | Cancel processing or a pending permission request |
| `$/cancel_request` | Cancel an in-flight request by its request ID |
| `session/load` | Restore saved context and replay user messages and agent updates before responding |
| `session/resume` | Restore saved context without replaying updates |
| `session/list` | List saved sessions, optionally filtered by working directory, with pagination |
| `session/close` | Cancel active work and release runtime resources while preserving saved history |
| `session/delete` | Remove a saved conversation from the session history |
| `session/set_mode` | Select Ask, Code, or Chat mode |
| `session/set_model` | Select a provider/model without changing global configuration |
| `session/set_config_option` | Select a model or reasoning effort for this conversation |

The initial session response includes modes and configuration selectors. The adapter discovers the catalogues of configured providers during session setup, with a five-second discovery budget. The active model and configured recent models remain available if a provider cannot be queried. Models are exposed through both `configOptions` and the legacy `models` session field; explicit model selection accepts `provider/model`. Reasoning values are `default`, `none`, `low`, `medium`, `high`, `xhigh`, and `max`; individual providers may impose narrower limits. `/help` and `/compact` are advertised as available commands. Model finish reasons are mapped to ACP completion, token-limit, and refusal stop reasons.

## Permissions, tools, and MCP

Code mode uses Solomon's existing execution policies. Chat mode selects Solomon's chat tool set. Mode changes require client requests.

Client-supplied MCP servers support stdio, streamable HTTP, and SSE. Their environment variables and headers are forwarded. Client definitions replace local definitions with the same name. Invalid definitions or client servers that fail to connect fail session creation instead of being silently ignored.

Filesystem access and command execution use Solomon's native tools. ACP's `fs/*` and `terminal/*` methods belong to the client API and are not agent methods that Solomon must expose. The adapter does not require these optional client capabilities.

## Persistence and boundaries

Conversations, provider/model selection, reasoning settings, mode, permission decisions, and replay history are stored under `$SOLOMON_HOME/acp/sessions` (normally `~/.solomon/acp/sessions`). Images and binary attachments are stored under `acp/attachments`. Session files are written atomically with owner-only permissions. Authentication uses existing Solomon provider credentials; missing configuration returns ACP's authentication-required error. No separate ACP login/logout method is advertised.

Prompt execution is serialized because existing Solomon tools use the process working directory. Each conversation retains independent context and cancellation remains responsive while another session is running. Transport errors are reported and disconnecting cancels active work.

Audio prompts are not advertised because Solomon's completion backend has no audio input path. Additional workspace roots and elicitation are likewise not advertised. Unknown methods return method-not-found; malformed requests, unsupported content, and invalid configuration values return protocol errors. ACP v2 is a draft and is not negotiated.

Specification: https://agentclientprotocol.com/protocol/v1/overview

## Branding

The classic Solomon crown is supplied without a background in `initialize.result.agentInfo._meta["solomon/branding"].icons`. The icon descriptor includes a self-contained PNG data URI, MIME type, and size; it needs no server or local path on the client. This is an implementation extension through ACP v1's `_meta` mechanism, not a standard icon field. Clients must explicitly support this extension to display it.

`solomon acp --icon` exports the transparent SVG to stdout. The same vector asset is available at `site/assets/mark.svg`. The existing application icons retain their backgrounds. T3 Code currently ignores branding supplied by local ACP agents, so these Solomon-side changes alone do not replace its generic ACP icon.

ACP reasoning changes update Solomon's saved preference for the selected provider/model, preserving other models and fast-mode settings. Switching models restores that model's saved preference and updates the client selector. The globally selected model is not changed by an ACP model switch. Environment-only providers retain preferences in the ACP session snapshot. ACP exposes per-model reasoning levels when known, otherwise the classic fallback; an unset effort resolves to an allowed level.

Reasoning capabilities are discovered automatically from model catalogue responses without a per-provider opt-in. The shared decoder accepts Codex supported_reasoning_levels, Anthropic capabilities.effort, OpenRouter reasoning.supported_efforts, and Cursor parameter definitions. Claude subscription discovery preserves the official SDK supportedEffortLevels; Cursor subscription discovery also reads declared level variants. Missing or unusable metadata remains unknown rather than implying support for every effort.

ACP and the GUI expose only declared levels for known models, including minimal where supported. Models explicitly lacking configurable effort have no reasoning slider or ACP effort option. Unknown models retain none, low, medium, high, xhigh, and max; their label includes (reasoning levels unknown), highlighted in red in Solomon's GUI. ACP clients control their own label styling. Switching models resolves an incompatible saved selection to an allowed provider default, medium when allowed, or the first available level. Provider default remains absent from the selector.
