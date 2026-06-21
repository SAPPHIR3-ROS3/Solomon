# TODO

Task ordinate con questa **priorità**: (1) **indipendenza** — prima le voci che non bloccano altre e non sono bloccate da prerequisiti interni salvo dove indicato; (2) **velocità di implementazione** e **facilità** relativa dentro ogni fascia; (3) **dipendenze esplicite** — **vault** prima di **auth ai major lab** che deve appoggiarci i token/chiavi.

---

## 1 — Oracolo

- **Stato:** non presente nel prodotto.
- **Cosa manca:** **aggiunta dell'Oracolo** — definire ruolo (consultazione, verifica, routing domande, output UX) e implementarlo nel flusso Solomon senza duplicare slash/tool esistenti.

---

## 2 — Deep research (stile Odysseus)

- **Stato:** **MVP parziale in corso** — presenti `internal/research` (loop multi-round), tool `deepResearch` / `researchStatus`, slash `/research` (`list`, `status`, `stop`, `delete`, `resume`), job JSON persistiti in `projects/<id>/research/`, report HTML con TLDR, progress in REPL, pause su errore LLM e resume, tracking fallimenti URL/search (no silenziosi), stats in list. Fonti **solo web** (via `webfetch` + `internal/search`). Fetch web migliorato (UA browser, retry 429/503, cookie jar, `[web_fetch]` in config).
- **Cosa manca:** integrazione **file progetto** e **MCP** oltre al web; budget **token/tempo** più esplicito in UX e config; stima costi vs `usage` API più chiara; qualità estrazione (readability, meno `low_quality`); retry URL fallite al resume; eventuale `robots.txt` / delay per host; polish report HTML; test end-to-end su job lunghi con LLM locale.
- **Dipende da:** credenziali ricerca web più sane dopo **§3** vault, ma non bloccante per un MVP.

---

## 3 — Vault sicuro (informazioni sensibili)

- **Stato:** API key e altri segreti rilevanti sono principalmente nel **TOML** di configurazione in chiaro; nessun **vault** dedicato né uso sistematico di **Keychain** (macOS), **Credential Manager** (Windows), **libsecret** (Linux), o equivalente unificato.
- **Cosa manca:** progettare e implementare un **vault sicuro** centralizzato per tutte le info sensibili (chiavi provider, token OAuth quando introdotti, credenziali per ricerca web o MCP dove applicabile); API di lettura a runtime senza esporre plaintext su disco oltre il necessario; **migrazione** guidata da config legacy; chiarezza su headless/CI e backup/ripristino senza falle.

---

## 4 — Autenticazione verso i major lab

- **Stato:** **ChatGPT Sub** via browser OAuth PKCE + callback locale (`internal/auth/openai/codex/`). **Claude Sub** via browser OAuth PKCE + callback locale (`internal/auth/anthropic/claudeoauth/`, stesso modello Pi: `localhost:53692/callback`, token su `platform.claude.com`). **Anthropic API key** e endpoint compatibili OpenAI. **Cursor API** con sidecar. Token OAuth ancora in TOML (non vault).
- **Cosa manca:**
  - **Claude Sub via Agent SDK sidecar** — percorso ToS-allineato alternativo al browser OAuth diretto (pattern `internal/integrations/cursor/`).
  - **Google AI** e altri lab: login nativo dove ha senso.
  - **Vault (§3)** per token/chiavi invece del solo TOML; refresh/rotazione già parziale su ChatGPT Sub e Claude Sub.
  - Tabella/documentazione lab supportati nativamente vs solo endpoint compatibili.

---

## 5 — LSP

- **Stato:** nessun aggancio LSP; Solomon resta **solo terminale** + tool file/shell/MCP.
- **Cosa manca (se lo vorrai):** un client LSP (anche minimale) che alimenti il contesto: diagnostiche, simboli, "go to definition", errori di compilazione nel buffer del workspace — senza dover aprire l'IDE.
- **Nota:** è un ampliamento netto di superficie (processo server, protocollo, caching); utile soprattutto per **errori e navigazione**, non per sostituire l'IDE.

---

## 6 — Memoria (MemPalace) e Obsidian

- **Stato:** non integrato; sessioni e contesto restano chat + file progetto come oggi.
- **Cosa manca:** layer di **memoria esterna** basato su MemPalace (o equivalente scelto), con regole di lettura/scrittura; **integrazione Obsidian** (vault path, note come artefatti, sync convenzioni link/path) e confini tra memoria di progetto vs memoria personale.

---

## 7 — Sicurezza

- **Stato:** `shell` è **comando reale** sulla macchina, nella working directory del progetto; `readFile`/`editFile` risolvono path senza **path jail** forte (path assoluti possono uscire dalla root; symlink/`..` non sono trattati come "cage" del workspace). MCP ha allow/deny per nome tool sul server, ma l'host resta potente.
- **Integrità stream SSE (fail-closed):** in [`internal/llm/stream/completion.go`](internal/llm/stream/completion.go), `StreamText` e `StreamAssistantTurn` abortiscono il turno se `ChatCompletionAccumulator.AddChunk` rifiuta un chunk (tipicamente `id` completion incoerente nello stesso stream). Nessun salvage di `ReasoningText`, content o usage in sessione — possibile forgery / jailbreak surface, stessa filosofia del rifiuto delle completion forgiate lato provider. Errore: `llm.ErrStreamAccumulatorRejected`. Test: [`test/stream_integrity_test.go`](test/stream_integrity_test.go). Output già stampato sul terminale prima dell'abort può restare visibile ma non viene persistito.
- **Cosa manca (tipico desiderata):** sandbox o policy (whitelist comandi, blocchi per operazioni distruttive, conferme dove serve); **vincolo percorsi** sotto `ProjRoot` risolvendo e verificando prefisso; limiti più chiari per output sensibili.
- **`intent`** su shell/edit è **solo metadati per il modello**, non un sistema di approvazioni.

---

## LOW PRIORITY

- **Web UI minimale:** Solomon è **solo terminale** (REPL + slash + tool); nessuna superficie HTTP/browser per sessioni o turni. **Cosa manca (opzionale):** web UI minimale che sfrutti le feature esistenti — avvio sessione, invio messaggi, stream risposta/reasoning, visualizzazione tool call e risultati, usage/token, immagini, slash essenziali; backend locale (es. API su loopback) sullo stesso runtime/agent senza duplicare logica; bind solo localhost di default; scope iniziale chat + stato, senza sostituire REPL per shell/edit finché non serve. **Dipende da:** **§7** Sicurezza (o policy equivalente) prima di esporre tool potenti oltre il TTY; ampliamento di superficie simile a LSP (§5).

- **`chzyer/readline` su Windows — sequenze ANSI estese nel prompt:** il parser ANSI di readline v1.5.1 (`ansi_windows.go`) tratta erroneamente i codici SGR `38`/`48` (true color / 256 color) come indici colore base 30–37 e va in panic (`index out of range [8]`). Workaround attuale: `termcolor.WrapUserReadline` usa solo sequenze basic (`\033[96m`) nei prompt passati a readline su Windows. **Cosa manca (opzionale):** patch upstream o fork di readline con supporto `38;2;…` / `38;5;…`, oppure sostituire readline con una libreria TTY cross-platform che gestisca il true color; finché resta readline, evitare lipgloss/true color su qualsiasi stringa che passa da `SetPrompt` / `Readline` su Windows.

- **Anthropic / extended thinking (dopo adapter Messages API v1):** oggi il piano Anthropic nativo prevede extended thinking **disattivato** e reasoning in API solo sull'ultimo messaggio `assistant`; in sessione resta `ReasoningText` per display. **Cosa manca:** abilitare `thinking` in request (`budget_tokens` / adaptive da config); persistere **`ThinkingBlocks`** (blocchi `thinking` + `signature` immutabile) su messaggi assistant in `chatstore`; mapper Anthropic che reinserisce i blocchi in history; rivalutare se la policy "solo ultimo assistant" basta per tool/multi-turn o serve history thinking completa; stream/usage per thinking tokens; documentare impatto token (prompt gonfio se si reinvia tutta la history). Dipende da: layer `CompletionBackend` + provider `api_protocol = anthropic`.

- **Ricerca semantica nel codice:** oggi `find` copre solo **glob + regexp** su file; nel bridge Cursor `SemanticSearch` → `find` con `pattern=query` è un **fallback testuale**, non semantica vera. **Cosa manca (opzionale):** tool dedicato (es. `semanticFind` / `codeSearch`), separato da `find`; indice del workspace (embeddings locali o API) con aggiornamento incrementale, rispetto `.gitignore`, limiti su file binari/segreti; query per concetti ("dove si gestisce l'auth?") con chunk + path/righe; integrazione build mode + dump/help + alias bridge Cursor verso il tool reale. **Approccio preferito:** provare prima via **MCP opzionale** o comando `/index` on-demand; nativizzare in core solo se diventa uso quotidiano. `find` resta il percorso deterministico per simboli/stringhe note.

- **Allineamento esperienza Windows con Linux/macOS:** oggi Solomon gira su Windows ma con compromessi e fork per OS (es. `termcolor.WrapUserReadline` e limiti ANSI readline, `/clear` assente in cmd.exe, input console/multiline, clipboard via PowerShell, rilevamento shell, banner e test spesso saltati su `GOOS=windows`). **Cosa manca:** audit delle divergenze UX tra piattaforme; parità per terminale interattivo (colori, pulizia schermo, paste immagini, hotkey REPL) su Windows Terminal e PowerShell; riduzione dei percorsi `*_windows.go` / `*_unix.go` dove fattibile; test e documentazione setup Windows allineati al flusso macOS/Linux.

- **REPL input multilinea — parità scroll con shell normale:** oggi l'editor custom in [`internal/agent/runtime/repl/editor`](internal/agent/runtime/repl/editor) è **usabile** (paste multilinea, wrap visivo, commit su Invio) ma non replica l'UX di una shell/readline nativa: con input più alto dello schermo il messaggio dovrebbe restare **intero nello scrollback del terminale** e lo scroll (rotella / freccia su) dovrebbe far scorrere la vista del terminale, non ridisegnare una "finestra" interna né corrompere la history (righe duplicate/sovrascritte quando si naviga oltre le ultime righe visibili). **Cosa manca (opzionale):** modello render che non faccia full-redraw oltre l'altezza del TTY; scroll naturale del buffer già stampato; niente scrollbar interna al blocco input; invariante "dopo Invio il testo inviato è immutabile". Stato attuale accettato come sufficiente; riprendere solo se serve parità totale con Ghostty/zsh su paste lunghi.

---

## EXTREMELY LOW PRIORITY

- **Chat mode — playbook/persona (tool dedicati, distinti da skill agent):** in **chat mode** potrebbero servire tool per cercare e caricare playbook conversazionali (persona, tono, formato risposta) senza riusare `searchSkill`/`loadSkill` dell'agent: le skill installate possono includere istruzioni orientate a tool/implementazione. **Cosa manca (opzionale):** pair dedicato in superficie chat (es. `searchPersona`/`loadPersona` o `searchPlaybook`/`loadPlaybook`), stesso registry `SKILL.md` sotto con filtro/tag frontmatter (`mode: chat | agent | both`) o criterio equivalente; dump/prompt chat; slash opzionale simmetrico se serve. Fuori scope chat: orchestrate e deferred build.

- **Wiki (`docs/`) — ampliamenti opzionali:** base consegnata e allineata al codice ([`docs/README.md`](docs/README.md), portali `user-guide/` / `architecture/` / `development/`, catalogo [`docs/features.md`](docs/features.md), README snellito). Per dopo, solo se serve: troubleshooting, contributing, GitHub Pages / MkDocs, check automatico documentazione ↔ codice in CI.
- **macOS — `Cmd+V` per incollare immagini nella REPL:** oggi su Mac l'unica hotkey funzionante per il paste immagine è `Ctrl+V` (intercettata dal listener readline in `internal/agent/runtime/repl.go`, `key == 22`). `Cmd+V` non è gestibile direttamente lato Solomon perché `Cmd` è un modificatore OS-level che gli emulatori di terminale (Terminal.app, Ghostty, iTerm2, ecc.) traducono in `paste_from_clipboard` testuale prima del PTY: se negli appunti c'è solo un'immagine senza rappresentazione testuale, al processo non arriva alcun byte. Soluzione tecnicamente valida e universale (funziona in ogni emulatore senza config utente): un helper basato su `CGEventTap` (`ApplicationServices`/`Quartz`) che intercetti `Cmd+V` a livello HID, verifichi che il foreground process del TTY corrente sia `solomon` (`tcgetpgrp` su `os.Stdout.Fd()`), consumi l'evento e posti un `Ctrl+V` sintetico via `CGEventPost`. Requisiti: codice nativo Swift/ObjC o cgo verso `ApplicationServices`/`Quartz`, permesso *Privacy & Security → Accessibility* concesso dall'utente, idealmente binario firmato/notarizzato per evitare friction Gatekeeper. Design proposto: feature **opt-in**, off di default, attivabile con uno slash command dedicato (es. `/cmdv enable`) che spieghi i passaggi richiesti, triggeri il prompt di sistema (`AXIsProcessTrustedWithOptions`) e persista il flag in config utente; allo startup della REPL, se il flag è on, controllo **una volta sola** del permesso e avvio dell'event tap legato al lifecycle del `Runtime.Run`, niente check ricorrenti su ogni evento clipboard. Alternative scartate: `hidutil` (è solo device-scoped, non app-scoped, agisce a livello IOKit dove la nozione di app frontale non esiste); rimappa per‑terminale via config (Ghostty `keybind = cmd+v=text:\x16` + `clipboard.PasteText` fallback nell'handler, oppure scorciatoia menu in Terminal.app) — funzionano ma richiedono setup manuale per ogni terminale e quindi non sono *Cmd+V universale*; auto‑attach via polling di `NSPasteboard.changeCount` (zero permessi ma cambia la UX, non è `Cmd+V`). Vincoli da rispettare quando si implementerà: rispetto della regola 500 LoC per file (l'event tap probabilmente richiede un file nuovo dedicato, es. `internal/clipboard/cmdv_darwin.go` + build tag, con stub no-op sugli altri OS), assenza di nuove dipendenze Go esterne, l'attivazione non deve alterare il comportamento di `Ctrl+V` esistente.
