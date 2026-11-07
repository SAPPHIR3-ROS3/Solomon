# Memory diagnostics

The daemon samples its Go heap and owned Linux processes every ten seconds. It
keeps the latest 60 samples in memory (about ten minutes) and an observed RSS peak
for the lifetime of the collector. `GET /__solomon/memory` returns this bounded
history through the existing daemon API. Nothing is persisted; external transmission occurs only through report submission.
Anonymous reports include the metrics as a JSON string in the `memory` field.
The complete serialized HTTP payload is limited to 48 KiB. Logs retain their
32 KiB raw allowance; the encoded memory string is limited to 16 KiB. Oldest
samples are removed as needed, preserving the observed peak and newest samples.
If JSON escaping and metadata overflow the total budget, the oldest log text is
trimmed on Unicode boundaries. Local diagnostics retain the full 60 samples.

The process groups are the daemon itself, desktop clients registered in this
Solomon home's lifecycle registry, and WebKit helpers descended from those
clients. Desktop registration must match the process creation identity and the
actual executable file; executable basenames are restricted to Solomon binaries.
Only processes belonging to the current OS user are measured. PID reuse, stale
registrations, and mismatched executables are rejected. The collector follows
WebKit and sandbox wrapper children, not arbitrary applications launched by the
desktop. It never selects all WebKit processes by name. Discovery is bounded to
256 processes and 16 descendant levels per scan.

This scope excludes external browsers, tool commands, provider CLIs, MCP servers,
Vite, and independent terminal clients. Only registered desktop clients are
included; an unregistered client is omitted rather than guessed. On non-Linux
systems the Go heap remains available and `processMetricsAvailable` is false.
Incomplete access to desktop helpers can undercount their usage. RSS is resident
memory, not virtual address space; summed RSS can count shared pages more than
once. Swap is reported separately. The peak is sampled, not an OS high-water mark.
This is footprint monitoring, not an allocation stack profiler or a heap dump.

The output schema contains only fixed group names, counts, byte values, GC
counts, and elapsed seconds. It contains no PIDs, process arguments, filesystem
paths, environment variables, user identifiers, chat content, or memory contents.

## Report budget

The approved serialized report budget is 48 KiB, including all fields and JSON
escaping. Memory metrics receive at most 16 KiB of encoded JSON string data.
Tests cover populated maximum counters, sample trimming, Unicode logs, and
escaping expansion. The remote receiver is outside this repository; its schema
and body limit have not been verified or changed. Local tests use a mock receiver
and never send diagnostic data to the production endpoint.

## Snapshot recursion fix

File change capture excludes every `solomon-changes-*` path component, including
explicit targets and resolved symlink aliases. Directory traversal prunes these
folders and Git discovery also filters them. Session loading removes contaminated
internal snapshot entries from ledgers and saved turn summaries, including branch
and archived messages, without deleting legitimate project changes. Existing
transcript files are not rewritten by inspection; the ordinary save lifecycle
persists the cleaned session when it next saves.

Sidebar listing streams only session metadata and message times, rather than
retaining complete transcripts and ledgers for every chat. A bounded metadata
cache is invalidated by file identity, size, or modification time. The first read still scans
the complete JSON file and may allocate individual large strings while skipping
values. Opening an old bloated transcript still needs its initial full decode;
this change does not perform an on-disk migration of the user's saved chats.
Subagent polling waits for each response and aborts requests on selection changes.
