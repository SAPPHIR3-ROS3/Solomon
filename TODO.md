# TODO

Task ordinate con questa **priorità**: (1) **indipendenza** — prima le voci che non bloccano altre e non sono bloccate da prerequisiti interni salvo dove indicato; (2) **velocità di implementazione** e **facilità** relativa dentro ogni fascia; (3) **dipendenze esplicite** — **vault** prima di **auth ai major lab** che deve appoggiarci i token/chiavi.

---


## 1 — Model routing

- **Stato:** scelta **manuale** di provider/modello (`/connect`, config); nessuna policy automatica per tipo di task, costo, fallback o degradazione controllata.
- **Cosa manca:** **miglior model routing** — regole o euristiche configurabili (es. modello leggero per passaggi meccanici, modello forte per refactor; fallback su errore rate-limit; profili nominati legati a contesto).

---

## 2 — Integrazione file completa

- **Stato:** `readFile` / `editFile` + `shell` coprono molti casi; non c’è un insieme esplicito e completo di operazioni file-first (es. rename/delete/list/glob come primitive dedicate, vincoli chiari sul workspace, coerenza con checkpoint).
- **Cosa manca:** superficie **file** omogenea e “completa” rispetto al flusso agente (operazioni mancanti, semantica unificata, allineamento con vincoli di path/sandbox quando saranno affrontati nella sezione **Sicurezza**).

---

## 3 — Code mode e altri tool

- **Stato:** modalità `plan` / `build` e set tool attuale; nessuna "code mode" dedicata o set esteso come da design desiderato.
- **Cosa manca:** definire **code mode** (tool permessi, system prompt, eventuale separazione da build); aggiungere gli **altri tool** concordati (nativi o via MCP) e aggiornare dump/help coerentemente.

---

## 4 — Persistenza subagent

- **Stato:** esistono **directory e helper** per `subchats` (`SubchatsDir`, `SubchatPath`), ma la run annidata costruisce soprattutto **transcript in memoria** e restituisce una stringa al parent; non c'è un **file di sessione subagent** completo e riapribile come la chat principale (messaggi, tool, usage, id stabile, resume). Il tool `subagent` espone solo `sysPromptPath` + `task`; reasoning nested è **sempre disabilitato** (`ForceDisableReasoning: true` in `nested.go`). Il bridge Cursor `Task` → `subagent` ignora `resume`, `interrupt`, `run_in_background` e non ha equivalente per `subagent_type` / `readonly`.
- **Cosa manca — persistenza:** modello di sessione allineato alla chat principale (stesso schema o sottoinsieme), **ID univoco** per sub-run (esposto al parent e riusabile come `resume`), salvataggio incrementale a ogni turno/tool, collegamento al messaggio/tool call che ha spawnato il subagent; riapertura con `/resume` o argomento `resume` sul tool.
- **Cosa manca — parametri `subagent` (allineati a Task Cursor, solo quelli utili):**
  - **`resume`** (opzionale): ID subchat da riprendere; richiede persistenza e lookup su `subchats/`.
  - **`interrupt`** (opzionale): interrompe una sub-run in corso e accetta il nuovo `task`; richiede tracking run attive (o cancel via context + id).
  - **`run_in_background`** (opzionale): subagent non blocca il parent; richiede run async, notifica/aggregazione risultato al parent (analogo Task background).
  - **`reasoningEffort`** (opzionale, default **`none`**): `none | low | med | high` solo per la run nested; sostituisce il hardcoded `ForceDisableReasoning: true` quando esplicitamente richiesto; la chat principale resta governata da `/reasoning` e config globale.
- **Schema tool:** estendere `subagentOpenAI()` / `execSubagent` e il bridge `legacy-normalize` (`Task` → `subagent`) per i campi sopra; aggiornare dump, display tool e test proxy.
- **Fuori scope (per ora):** `subagent_type` e `readonly` (Cursor-specific, nessun equivalente Solomon); **`model`** e **`file_attachments`** — non implementare finché non c’è policy esplicita (model nested, allegati per sub-run vs `ImageFiles` di sessione).

---

## 5 — Oracolo

- **Stato:** non presente nel prodotto.
- **Cosa manca:** **aggiunta dell’Oracolo** — definire ruolo (consultazione, verifica, routing domande, output UX) e implementarlo nel flusso Solomon senza duplicare slash/tool esistenti.

---

## 6 — Vault sicuro (informazioni sensibili)

- **Stato:** API key e altri segreti rilevanti sono principalmente nel **TOML** di configurazione in chiaro; nessun **vault** dedicato né uso sistematico di **Keychain** (macOS), **Credential Manager** (Windows), **libsecret** (Linux), o equivalente unificato.
- **Cosa manca:** progettare e implementare un **vault sicuro** centralizzato per tutte le info sensibili (chiavi provider, token OAuth quando introdotti, credenziali per ricerca web o MCP dove applicabile); API di lettura a runtime senza esporre plaintext su disco oltre il necessario; **migrazione** guidata da config legacy; chiarezza su headless/CI e backup/ripristino senza falle.

---

## 7 — Autenticazione verso i major lab

- **Stato:** provider configurati in TOML con **base URL OpenAI-compatibile** e **API key** in chiaro (`internal/config`); client costruito con `option.WithAPIKey` / `WithBaseURL` (`internal/agent/runtime.go`). Nessun flusso OAuth né integrazione dedicata per singoli vendor.
- **Cosa manca:** dove ha senso tecnico e legale, **auth ufficiale** verso i provider principali (OpenAI, Anthropic, Google AI, ecc.): gestione credenziali (incluso refresh o rotazione dove previsto), UX di login/chiave, profili multipli; appoggio al **vault** per token e chiavi invece del solo TOML; tabella/documentazione di quali lab sono supportati nativamente vs solo endpoint compatibili.

---

## 8 — LSP

- **Stato:** nessun aggancio LSP; Solomon resta **solo terminale** + tool file/shell/MCP.
- **Cosa manca (se lo vorrai):** un client LSP (anche minimale) che alimenti il contesto: diagnostiche, simboli, "go to definition", errori di compilazione nel buffer del workspace — senza dover aprire l'IDE.
- **Nota:** è un ampliamento netto di superficie (processo server, protocollo, caching); utile soprattutto per **errori e navigazione**, non per sostituire l'IDE.

---

## 9 — Memoria (MemPalace) e Obsidian

- **Stato:** non integrato; sessioni e contesto restano chat + file progetto come oggi.
- **Cosa manca:** layer di **memoria esterna** basato su MemPalace (o equivalente scelto), con regole di lettura/scrittura; **integrazione Obsidian** (vault path, note come artefatti, sync convenzioni link/path) e confini tra memoria di progetto vs memoria personale.

---

## 10 — Sicurezza

- **Stato:** `shell` è **comando reale** sulla macchina, nella working directory del progetto; `readFile`/`editFile` risolvono path senza **path jail** forte (path assoluti possono uscire dalla root; symlink/`..` non sono trattati come "cage" del workspace). MCP ha allow/deny per nome tool sul server, ma l'host resta potente.
- **Integrità stream SSE (fail-closed):** in [`internal/llm/stream.go`](internal/llm/stream.go), `StreamText` e `StreamAssistantTurn` abortiscono il turno se `ChatCompletionAccumulator.AddChunk` rifiuta un chunk (tipicamente `id` completion incoerente nello stesso stream). Nessun salvage di `ReasoningText`, content o usage in sessione — possibile forgery / jailbreak surface, stessa filosofia del rifiuto delle completion forgiate lato provider. Errore: `llm.ErrStreamAccumulatorRejected`. Test: [`test/stream_integrity_test.go`](test/stream_integrity_test.go). Output già stampato sul terminale prima dell’abort può restare visibile ma non viene persistito.
- **Cosa manca (tipico desiderata):** sandbox o policy (whitelist comandi, blocchi per operazioni distruttive, conferme dove serve); **vincolo percorsi** sotto `ProjRoot` risolvendo e verificando prefisso; limiti più chiari per output sensibili.
- **`intent`** su shell/edit è **solo metadati per il modello**, non un sistema di approvazioni.

---

## LOW PRIORITY

- **`chzyer/readline` su Windows — sequenze ANSI estese nel prompt:** il parser ANSI di readline v1.5.1 (`ansi_windows.go`) tratta erroneamente i codici SGR `38`/`48` (true color / 256 color) come indici colore base 30–37 e va in panic (`index out of range [8]`). Workaround attuale: `termcolor.WrapUserReadline` usa solo sequenze basic (`\033[96m`) nei prompt passati a readline su Windows. **Cosa manca (opzionale):** patch upstream o fork di readline con supporto `38;2;…` / `38;5;…`, oppure sostituire readline con una libreria TTY cross-platform che gestisca il true color; finché resta readline, evitare lipgloss/true color su qualsiasi stringa che passa da `SetPrompt` / `Readline` su Windows.

- **Anthropic / extended thinking (dopo adapter Messages API v1):** oggi il piano Anthropic nativo prevede extended thinking **disattivato** e reasoning in API solo sull’ultimo messaggio `assistant`; in sessione resta `ReasoningText` per display. **Cosa manca:** abilitare `thinking` in request (`budget_tokens` / adaptive da config); persistere **`ThinkingBlocks`** (blocchi `thinking` + `signature` immutabile) su messaggi assistant in `chatstore`; mapper Anthropic che reinserisce i blocchi in history; rivalutare se la policy “solo ultimo assistant” basta per tool/multi-turn o serve history thinking completa; stream/usage per thinking tokens; documentare impatto token (prompt gonfio se si reinvia tutta la history). Dipende da: layer `CompletionBackend` + provider `api_protocol = anthropic`.

- **Ricerca semantica nel codice:** oggi `find` copre solo **glob + regexp** su file; nel bridge Cursor `SemanticSearch` → `find` con `pattern=query` è un **fallback testuale**, non semantica vera. **Cosa manca (opzionale):** tool dedicato (es. `semanticFind` / `codeSearch`), separato da `find`; indice del workspace (embeddings locali o API) con aggiornamento incrementale, rispetto `.gitignore`, limiti su file binari/segreti; query per concetti (“dove si gestisce l’auth?”) con chunk + path/righe; integrazione build mode + dump/help + alias bridge Cursor verso il tool reale. **Approccio preferito:** provare prima via **MCP opzionale** o comando `/index` on-demand; nativizzare in core solo se diventa uso quotidiano. `find` resta il percorso deterministico per simboli/stringhe note.

- **Allineamento esperienza Windows con Linux/macOS:** oggi Solomon gira su Windows ma con compromessi e fork per OS (es. `termcolor.WrapUserReadline` e limiti ANSI readline, `/clear` assente in cmd.exe, input console/multiline, clipboard via PowerShell, rilevamento shell, banner e test spesso saltati su `GOOS=windows`). **Cosa manca:** audit delle divergenze UX tra piattaforme; parità per terminale interattivo (colori, pulizia schermo, paste immagini, hotkey REPL) su Windows Terminal e PowerShell; riduzione dei percorsi `*_windows.go` / `*_unix.go` dove fattibile; test e documentazione setup Windows allineati al flusso macOS/Linux.

---

## EXTREMELY LOW PRIORITY

- **Wiki (`docs/`) — ampliamenti opzionali:** base consegnata e allineata al codice ([`docs/README.md`](docs/README.md), portali `user-guide/` / `architecture/` / `development/`, catalogo [`docs/features.md`](docs/features.md), README snellito). Per dopo, solo se serve: troubleshooting, contributing, GitHub Pages / MkDocs, check automatico documentazione ↔ codice in CI.
- **macOS — `Cmd+V` per incollare immagini nella REPL:** oggi su Mac l'unica hotkey funzionante per il paste immagine è `Ctrl+V` (intercettata dal listener readline in `internal/agent/runtime/repl.go`, `key == 22`). `Cmd+V` non è gestibile direttamente lato Solomon perché `Cmd` è un modificatore OS-level che gli emulatori di terminale (Terminal.app, Ghostty, iTerm2, ecc.) traducono in `paste_from_clipboard` testuale prima del PTY: se negli appunti c'è solo un'immagine senza rappresentazione testuale, al processo non arriva alcun byte. Soluzione tecnicamente valida e universale (funziona in ogni emulatore senza config utente): un helper basato su `CGEventTap` (`ApplicationServices`/`Quartz`) che intercetti `Cmd+V` a livello HID, verifichi che il foreground process del TTY corrente sia `solomon` (`tcgetpgrp` su `os.Stdout.Fd()`), consumi l'evento e posti un `Ctrl+V` sintetico via `CGEventPost`. Requisiti: codice nativo Swift/ObjC o cgo verso `ApplicationServices`/`Quartz`, permesso *Privacy & Security → Accessibility* concesso dall'utente, idealmente binario firmato/notarizzato per evitare friction Gatekeeper. Design proposto: feature **opt-in**, off di default, attivabile con uno slash command dedicato (es. `/cmdv enable`) che spieghi i passaggi richiesti, triggeri il prompt di sistema (`AXIsProcessTrustedWithOptions`) e persista il flag in config utente; allo startup della REPL, se il flag è on, controllo **una volta sola** del permesso e avvio dell'event tap legato al lifecycle del `Runtime.Run`, niente check ricorrenti su ogni evento clipboard. Alternative scartate: `hidutil` (è solo device-scoped, non app-scoped, agisce a livello IOKit dove la nozione di app frontale non esiste); rimappa per‑terminale via config (Ghostty `keybind = cmd+v=text:\x16` + `clipboard.PasteText` fallback nell'handler, oppure scorciatoia menu in Terminal.app) — funzionano ma richiedono setup manuale per ogni terminale e quindi non sono *Cmd+V universale*; auto‑attach via polling di `NSPasteboard.changeCount` (zero permessi ma cambia la UX, non è `Cmd+V`). Vincoli da rispettare quando si implementerà: rispetto della regola 500 LoC per file (l'event tap probabilmente richiede un file nuovo dedicato, es. `internal/clipboard/cmdv_darwin.go` + build tag, con stub no-op sugli altri OS), assenza di nuove dipendenze Go esterne, l'attivazione non deve alterare il comportamento di `Ctrl+V` esistente.
