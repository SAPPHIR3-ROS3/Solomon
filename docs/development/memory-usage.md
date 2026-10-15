# Memory measurements

Measured on 2026-10-15 with Node 20.19.2, Vite 8.1.5, Linux and 16 available
logical CPUs. Values describe these local runs, rather than memory limits.

The baseline already has selective dependency prebundling in `vite.config.ts`.
Each measurement uses a separate Vite process and an isolated headless browser.
Cold runs use a new temporary dependency cache; warm runs reuse that cache in a
new process. Browser JavaScript heap values come from `Runtime.getHeapUsage`;
they are not the total RAM used by Chrome. Server values are RSS from Node.

| Measurement | Before these changes | After |
| --- | ---: | ---: |
| Vite RSS, cold cache, home loaded | 831 MiB | 597 MiB |
| Vite RSS, cold cache, five languages loaded | 853 MiB | 632 MiB |
| Modules registered by Vite on home | 347 | 267 |
| Modules registered after a plain code fence | 589 | 267 |
| Browser JS heap added by a plain code fence | about 10 MiB | below 0.1 MiB |
| Engine requests caused by a plain code fence | 2 | 0 |

The cold-start reduction comes primarily from limiting Rolldown's Rayon worker
pool to two threads. The development daemon and `npm run dev` set this default
before loading the compiler. A nonempty `RAYON_NUM_THREADS` setting is preserved.
Production builds retain their normal parallelism.

Plain code fences and unknown file types no longer initialize Shiki or its
Oniguruma WASM engine. Real languages still initialize the engine when needed.
The editor keeps its default foreground color without allocating per-token
decorations for plain files. The embedded documentation is imported only when
the Docs section is opened.

Warm-cache RSS varied by tens of MiB between runs, including due to garbage
collection and retained native allocator memory. Do not infer a fixed percentage
reduction or an upper bound from one RSS sample. Loading real languages and
opening documentation will still consume additional memory.

Disabling development source maps or removing the inline WASM module from
prebundling did not produce a consistent overall saving in the trials and was
not retained. The backend remained around 70 MiB at rest; no forced garbage
collection or backend memory cap was introduced.

Relevant checks:

```sh
go test ./test -run TestViteWorkerBudgetAndArgumentForwarding -count=1
cd gui
node --test e2e/syntaxMemory.test.mjs tests/settingsLocation.test.mjs
npm run build
SOLOMON_TEST_BINARY=/path/to/a/newly-built/solomon node --test e2e/mobile.test.mjs
```

The syntax browser checks verify that plain text does not request an engine,
that LF/CRLF content and token offsets are preserved, that real grammars still
work, and that the embedded manual loads only after opening Docs.
