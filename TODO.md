# TODO

Task ordinate con questa **priorità**: (1) **indipendenza** — prima le voci che non bloccano altre e non sono bloccate da prerequisiti interni salvo dove indicato; (2) **velocità di implementazione** e **facilità** relativa dentro ogni fascia; (3) **dipendenze esplicite** — **pattern immagini Unicode** prima dei **template** che descrivono il flusso immagini agli LLM; **vault** prima di **auth ai major lab** che deve appoggiarci i token/chiavi.

---

## Bugs da risolvere

Ordine suggerito: dal più **facile** al più **difficile** (code review interno).

- **`internal/clipboard/clipboard.go` — paste immagine Windows:** script PowerShell senza `Add-Type -AssemblyName System.Windows.Forms`; può fallire a runtime.
- **`internal/skills/registry.go` — lock registry:** `flock.TryLockContext` con `context.Background()` e delay lungo come “timeout” effettivo → retry potenzialmente indefinito; usare contesto con deadline e retry breve.
- **`internal/agent/runtime/repl.go` — paste clipboard:** errori (directory immagini, `PasteImage`, ecc.) assorbiti in silenzio e UX incerta (es. carattere stray nel buffer); feedback esplicito su stdout/stderr.
- **`internal/agent/commands/summarize.go`:** rischio di salvare escape ANSI nel contenuto del summary se l’output colorato finisce nel testo persistito.
- **`internal/agent/tools/exec.go`:** ramo `switch` morto o ridondante sul dispatch tool (pulizia dopo verifica comportamento atteso).
- **`internal/llm/stream.go`:** se l’accumulatore rifiuta un chunk di reasoning, testo reasoning può andare perso (edge case stream/provider).
- **`internal/agent/runtime/deferred_chat_title.go` (+ turn loop):** possibile race tra aggiornamenti async della sessione e persistenza / append messaggi; allineare alla decisione di estendere `chatPersistMu` (vedi anche TODO architettura compaction).

---

## 1 — Wiki

- **Stato:** documentazione dispersa tra `README` e codice; nessuna **wiki** dedicata (hosting GitHub/GitLab, sito statico, o spazio strutturato con indice e pagine collegate).
- **Cosa manca:** definire **ambito** (setup, architettura, comandi, contribuzione, troubleshooting); creare la wiki (repo `wiki`, Pages, o altra piattaforma scelta); tenere allineati README minimi vs approfondimenti in wiki per evitare doppioni o contenuti obsoleti.

---

## 2 — Integrazione nelle CI (come Codex / Claude: output strutturato)

- **Stato:** `solomon` / `exec` sono orientati al **TTY**; nessuna modalità **first-class** per usarlo come step in **GitHub Actions, GitLab CI, Buildkite**, ecc. con **stream o summary in JSON** (eventi per tool, messaggi, errori, esito) consumabile da script o dashboard, sul modello di ciò che offrono harness tipo **Codex** o **Claude** in pipeline.
- **Cosa manca:** contratto di **output machine-readable** (es. JSONL o JSON aggregato a fine run), opzioni esplicite **no-color / non-interactive**, **exit code** e segnalazione fallimenti prevedibili, documentazione ed esempi di job CI (segreti da env, working directory repo, limiti timeout); eventuale stream di eventi allineato a un **schema** stabile per integrazioni.

---

## 3 — Font e colori (terminale robusto)

- **Stato:** output colorato tramite escape ANSI (`termcolor` e affini), spesso assumendo capacità terminale omogenee (truecolor/monospace larghezza); niente contratto esplicito con `NO_COLOR` / tema chiaro-scuro / font che alterano metriche dei caratteri o grapheme composti.
- **Cosa manca:** comportamento **più robusto** tra ambienti reali — rispetto `NO_COLOR`, fallback a palette ridotta o plain quando non è un TTY o il terminale è limitato; contrasto leggibile e coerenza su Windows/macOS/Linux; gestione predicibile con pipe e log; orientamento utente su **font monospace** affidabili e su limiti noti delle emoji/simboli in REPL/tool output.

---

## 4 — Byte truncation (limiti output)

- **Stato:** `readFile` e output `shell` sono restituiti **interi** come stringa; nessun tetto configurabile per byte/righe lato harness.
- **Cosa manca:** limiti globali o per-tool (max bytes, max righe, tail/head); indicazione esplicita "troncato" + conteggio omesso; stesso principio opzionale per risultati MCP molto grandi.

---

## 5 — Integrazione `AGENTS.md` / `CLAUDE.md` (istruzioni progetto)

- **Stato:** il contesto di sistema è guidato da template in `internal/prompt` e configurazione utente; non c’è discovery automatica né fusione con file di convenzione nella **root del repository** (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md` o analoghi usati da IDE/assistenti).
- **Cosa manca:** rilevare e leggere questi file (percorso, priorità, merge con skill/rules esistenti); includerli nel system prompt o in un blocco dedicato senza duplicare inutilmente i template; documentare il comportamento atteso per chi mantiene il progetto.

---

## 6 — Robustezza API

- **Stato:** chiamate HTTP/stream verso endpoint OpenAI-compatibile senza uno strato evidente di **retry con backoff**, **circuit breaker** o distinzione fine tra errori retryable vs permanenti nel pacchetto LLM.
- **Cosa manca:** politica configurabile di retry per 429/5xx/disconnect durante stream; jitter; eventualmente timeout distinti per "connect" vs "read body"; logging strutturato dell'errore per diagnosi.

---

## 7 — Tab completion

- **Stato:** readline senza completamento strutturato su comandi slash, path workspace, nomi sessione, modelli, skill o altri elenchi noti all’ harness.
- **Cosa manca:** completion contestuale (tab) per ridurre errori e accelerare `/comandi`, percorsi file, dove applicabile provider/modello.

---

## 8 — Input multiline

- **Stato:** da valutare rispetto al loop readline attuale (una riga / invio = invio messaggio o comportamento limitato al multiline).
- **Cosa manca:** UX e implementazione chiare per incollare o scrivere **blocchi su più righe** senza invio prematuro (delimitatori, modalità "paste", shortcut, o editor esterno) e coerenza con persistenza messaggi.

---

## 9 — Autosuggest dalla history

- **Stato:** history readline standard; nessun suggerimento proattivo basato su **storico sessione/progetto** (né fuzzy match su input precedenti).
- **Cosa manca:** autosuggestion stile shell moderna (completamento anteprima da history locale/progetto) dove compatibile con il loop multiline e lo slash dispatch.

---

## 10 — Syntax highlighting nel terminale

- **Stato:** output REPL e risultati tool sono testo grezzo (colori dove già usati, es. `termcolor`); nessun highlight di linguaggio su blocchi di codice o comandi durante digitazione/visualizzazione.
- **Cosa manca:** evidenziazione sintattica coerente (es. paste, anteprima snippet, output assistant) integrata col terminale in uso, senza rompere copy-paste o accessibilità.

---

## 11 — Pattern immagini rafforzato (caratteri invisibili)

- **Stato:** placeholder visibili tipo `[img-n]` nel testo utente; rischio collisione o stripping ambiguo.
- **Cosa manca:** **delimitazione robusta** con sequenze Unicode invisibili (es. ZWJ/ZWSP o marker dedicati) attorno ai token immagine, parser lato harness che riconosce solo quel pattern, migrazione/dual-read per sessioni vecchie se necessario.

---

## 12 — Template e immagini

- **Stato:** i template in `internal/prompt` (plan/build/title/summarize) non incorporano esplicitamente il flusso **immagini** / placeholder sessione.
- **Cosa manca:** aggiornare i **template** affinché instruiscano il modello su `[img-n]`, allegati, e uso coerente con `ImageFiles` / paste; allineare prompt di sistema al comportamento reale del runtime (**da fare dopo** la sintassi/parsing immagini robusto nella sezione precedente).

---

## 13 — Model routing

- **Stato:** scelta **manuale** di provider/modello (`/connect`, config); nessuna policy automatica per tipo di task, costo, fallback o degradazione controllata.
- **Cosa manca:** **miglior model routing** — regole o euristiche configurabili (es. modello leggero per passaggi meccanici, modello forte per refactor; fallback su errore rate-limit; profili nominati legati a contesto).

---

## 14 — Integrazione file completa

- **Stato:** `readFile` / `editFile` + `shell` coprono molti casi; non c’è un insieme esplicito e completo di operazioni file-first (es. rename/delete/list/glob come primitive dedicate, vincoli chiari sul workspace, coerenza con checkpoint).
- **Cosa manca:** superficie **file** omogenea e “completa” rispetto al flusso agente (operazioni mancanti, semantica unificata, allineamento con vincoli di path/sandbox quando saranno affrontati nella sezione **Sicurezza**).

---

## 15 — Code mode e altri tool

- **Stato:** modalità `plan` / `build` e set tool attuale; nessuna "code mode" dedicata o set esteso come da design desiderato.
- **Cosa manca:** definire **code mode** (tool permessi, system prompt, eventuale separazione da build); aggiungere gli **altri tool** concordati (nativi o via MCP) e aggiornare dump/help coerentemente.

---

## 16 — Persistenza subagent

- **Stato:** esistono **directory e helper** per `subchats` (`SubchatsDir`, `SubchatPath`), ma la run annidata costruisce soprattutto **transcript in memoria** e restituisce una stringa al parent; non c'è un **file di sessione subagent** completo e riapribile come la chat principale (messaggi, tool, usage, id stabile, resume).
- **Cosa manca:** modello di persistenza allineato alla chat (stesso schema o sottoinsieme), ID univoco per sub-run, salvataggio incrementale a ogni turno/tool, eventualmente collegamento al messaggio/tool call che ha spawnato il subagent.

---

## 17 — Oracolo

- **Stato:** non presente nel prodotto.
- **Cosa manca:** **aggiunta dell’Oracolo** — definire ruolo (consultazione, verifica, routing domande, output UX) e implementarlo nel flusso Solomon senza duplicare slash/tool esistenti.

---

## 18 — Vault sicuro (informazioni sensibili)

- **Stato:** API key e altri segreti rilevanti sono principalmente nel **TOML** di configurazione in chiaro; nessun **vault** dedicato né uso sistematico di **Keychain** (macOS), **Credential Manager** (Windows), **libsecret** (Linux), o equivalente unificato.
- **Cosa manca:** progettare e implementare un **vault sicuro** centralizzato per tutte le info sensibili (chiavi provider, token OAuth quando introdotti, credenziali per ricerca web o MCP dove applicabile); API di lettura a runtime senza esporre plaintext su disco oltre il necessario; **migrazione** guidata da config legacy; chiarezza su headless/CI e backup/ripristino senza falle.

---

## 19 — Autenticazione verso i major lab

- **Stato:** provider configurati in TOML con **base URL OpenAI-compatibile** e **API key** in chiaro (`internal/config`); client costruito con `option.WithAPIKey` / `WithBaseURL` (`internal/agent/runtime.go`). Nessun flusso OAuth né integrazione dedicata per singoli vendor.
- **Cosa manca:** dove ha senso tecnico e legale, **auth ufficiale** verso i provider principali (OpenAI, Anthropic, Google AI, ecc.): gestione credenziali (incluso refresh o rotazione dove previsto), UX di login/chiave, profili multipli; appoggio al **vault** per token e chiavi invece del solo TOML; tabella/documentazione di quali lab sono supportati nativamente vs solo endpoint compatibili.

---

## 20 — LSP

- **Stato:** nessun aggancio LSP; Solomon resta **solo terminale** + tool file/shell/MCP.
- **Cosa manca (se lo vorrai):** un client LSP (anche minimale) che alimenti il contesto: diagnostiche, simboli, "go to definition", errori di compilazione nel buffer del workspace — senza dover aprire l'IDE.
- **Nota:** è un ampliamento netto di superficie (processo server, protocollo, caching); utile soprattutto per **errori e navigazione**, non per sostituire l'IDE.

---

## 21 — Memoria (MemPalace) e Obsidian

- **Stato:** non integrato; sessioni e contesto restano chat + file progetto come oggi.
- **Cosa manca:** layer di **memoria esterna** basato su MemPalace (o equivalente scelto), con regole di lettura/scrittura; **integrazione Obsidian** (vault path, note come artefatti, sync convenzioni link/path) e confini tra memoria di progetto vs memoria personale.

---

## 22 — Sicurezza

- **Stato:** `shell` è **comando reale** sulla macchina, nella working directory del progetto; `readFile`/`editFile` risolvono path senza **path jail** forte (path assoluti possono uscire dalla root; symlink/`..` non sono trattati come "cage" del workspace). MCP ha allow/deny per nome tool sul server, ma l'host resta potente.
- **Cosa manca (tipico desiderata):** sandbox o policy (whitelist comandi, blocchi per operazioni distruttive, conferme dove serve); **vincolo percorsi** sotto `ProjRoot` risolvendo e verificando prefisso; limiti più chiari per output sensibili.
- **`intent`** su shell/edit è **solo metadati per il modello**, non un sistema di approvazioni.

---

## EXTREMELY LOW PRIORITY

- **macOS — `Cmd+V` per incollare immagini nella REPL:** oggi su Mac l'unica hotkey funzionante per il paste immagine è `Ctrl+V` (intercettata dal listener readline in `internal/agent/runtime/repl.go`, `key == 22`). `Cmd+V` non è gestibile direttamente lato Solomon perché `Cmd` è un modificatore OS-level che gli emulatori di terminale (Terminal.app, Ghostty, iTerm2, ecc.) traducono in `paste_from_clipboard` testuale prima del PTY: se negli appunti c'è solo un'immagine senza rappresentazione testuale, al processo non arriva alcun byte. Soluzione tecnicamente valida e universale (funziona in ogni emulatore senza config utente): un helper basato su `CGEventTap` (`ApplicationServices`/`Quartz`) che intercetti `Cmd+V` a livello HID, verifichi che il foreground process del TTY corrente sia `solomon` (`tcgetpgrp` su `os.Stdout.Fd()`), consumi l'evento e posti un `Ctrl+V` sintetico via `CGEventPost`. Requisiti: codice nativo Swift/ObjC o cgo verso `ApplicationServices`/`Quartz`, permesso *Privacy & Security → Accessibility* concesso dall'utente, idealmente binario firmato/notarizzato per evitare friction Gatekeeper. Design proposto: feature **opt-in**, off di default, attivabile con uno slash command dedicato (es. `/cmdv enable`) che spieghi i passaggi richiesti, triggeri il prompt di sistema (`AXIsProcessTrustedWithOptions`) e persista il flag in config utente; allo startup della REPL, se il flag è on, controllo **una volta sola** del permesso e avvio dell'event tap legato al lifecycle del `Runtime.Run`, niente check ricorrenti su ogni evento clipboard. Alternative scartate: `hidutil` (è solo device-scoped, non app-scoped, agisce a livello IOKit dove la nozione di app frontale non esiste); rimappa per‑terminale via config (Ghostty `keybind = cmd+v=text:\x16` + `clipboard.PasteText` fallback nell'handler, oppure scorciatoia menu in Terminal.app) — funzionano ma richiedono setup manuale per ogni terminale e quindi non sono *Cmd+V universale*; auto‑attach via polling di `NSPasteboard.changeCount` (zero permessi ma cambia la UX, non è `Cmd+V`). Vincoli da rispettare quando si implementerà: rispetto della regola 500 LoC per file (l'event tap probabilmente richiede un file nuovo dedicato, es. `internal/clipboard/cmdv_darwin.go` + build tag, con stub no-op sugli altri OS), assenza di nuove dipendenze Go esterne, l'attivazione non deve alterare il comportamento di `Ctrl+V` esistente.
