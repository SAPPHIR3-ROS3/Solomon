# Diagnostic logging

Solomon writes operational diagnostics to the existing daily files under
`~/.solomon/logs/`. Entries use `[DIAGNOSTIC]` and include their severity in
`severity`, a precise timestamp, and the emitting `source_pid`. They respect the configured log level and do not create GUI
notifications. The existing seven-day retention remains in effect.

## Investigation fields

- HTTP failures include a generated `request_id`, method, registered route,
  operation, status, elapsed time, response size, and available error context.
  `X-Solomon-Request-ID` returns the same identifier to the client. Request
  query strings, headers, bodies, and response bodies are not recorded.
- Failed, canceled, timed-out, or slow subprocesses include their operation,
  executable name, PID, exit code, process state, duration, and context error.
  Arguments, shell scripts, environment variables, and command output are
  excluded. Successful subprocesses shorter than one second remain quiet.
- Git baseline subprocesses inherit the HTTP request identifier. This connects
  an editor request to a specific Git PID and distinguishes cancellation from
  the five-second baseline deadline.
- On Linux, signal exits also record available system memory, parent RSS and
  swap, and cumulative kernel and cgroup OOM counters when readable. Counters
  are evidence to correlate with the event; a nonzero historical counter alone
  does not establish the cause of a particular exit.
- Managed process shutdown paths record the PID and requested termination
  reason before signaling the process.
- HTTP server failures include the latest available profiler RSS and Go heap
  sample and the observed RSS peak. These are sampled values, not a heap dump
  captured at the instant of failure.
- Outbound HTTP diagnostics include method, host, status, duration, error type,
  cancellation, and route-change context. URLs, credentials, headers, bodies,
  and error messages that could embed a URL are excluded.
- Chat lifecycle, stream write failures, rejected chat cache entries,
  configuration persistence stages, ACP protocol/output failures, sidecar
  exits, and desktop proxy fallback have explicit diagnostic events.

## GUI and protocol coverage

The desktop and ACP entry points initialize persistent logging independently
of the interactive CLI. GUI runtime exceptions, unhandled promise rejections,
React root errors, and selected background failures send metadata to the local
daemon. The payload contains an error class, stage, asset basename and numeric
location, never an exception message or document content.

GUI delivery is limited to ten events per minute per client and twenty per
minute at the daemon. Payloads are limited to 1 KiB. Delivery uses a three-second
timeout and does not retry recursively. If the daemon is unavailable, the
browser console receives the same metadata. Expected aborts remain quiet.
Healthy HTTP polling remains quiet; mutations and requests taking at least one
second are logged. Diagnostic strings are bounded, made single-line, and
passed through credential and personal-data redaction.

Logging storage failures have a rate-limited stderr fallback, without calling
the logger recursively. This covers directory creation, opening and writing
the notification journal, and writing or rotating daily logs.

## Repository audit

Run `python3 scripts/logging/audit.py` from the repository. It scans owned Go,
TypeScript, JavaScript, Python, shell and PowerShell sources, including the CLI,
backend, desktop, GUI, integrations, TUI, prototype, site and build scripts.
Generated code, bundled assets, dependencies and tests are excluded. It reports
per-directory inventory and candidates involving subprocesses without local
logging, ignored results, or empty exception handlers.

The initial audit covered approximately 1,100 files. Candidates were reviewed
by their observation boundary: HTTP handlers, tool dispatch, provider retry
loops, persistence entry points, subprocess supervision, and standalone command
stderr. Pure transformations do not need their own logs when their callers
already record the failure. Hash writes, normal EOF, optional availability
probes, best-effort response-body closes, and cleanup after an already reported
failure intentionally remain quiet. Promise handlers that settle a queue after
the original caller receives its rejection are not independent error losses.

The inventory is heuristic and remains useful after this pass; a reported
candidate is not automatically a defect, and its absence does not prove that
every possible failure is observable. Diagnostic changes require a new build
and normal application restart to affect already running binaries.
