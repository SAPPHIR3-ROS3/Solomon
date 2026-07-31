# Audit tecnico — Solomon

> **File locale:** non aggiungere `AUDIT-FABLE.md` a Git, non committarlo e non pubblicarlo.

> Read-only, 13 giugno 2026, commit `e9a6a72`. Riferimenti file:riga verificati sul codice reale. Riverifica del 13/06 notte: punti 3 e 4 del debito implicito risolti, punto 2 in gran parte risolto — dettagli nelle rispettive sezioni.

## Contesto di lettura

Questo audit è calibrato su tre fatti: il progetto è di un **singolo sviluppatore**, che usa **Go da inizio anno**, e il codice è stato **in gran parte generato** — architettura e design pensati nel dettaglio dall'autore, ma il codice stesso letto solo per skimming. È inoltre prevista (potenzialmente) una **revisione completa del codice a valle dei TODO**. Queste premesse non cambiano i fatti trovati, ma cambiano cosa è sorprendente e cosa no: i problemi rilevati hanno la firma tipica del codice generato — pattern completi ma non collegati, errori zittiti per far compilare — e sono per costruzione invisibili allo skimming. Sono esattamente le cose che un audit deve cercare in un progetto così.

## In breve

Stato di salute: **B-**, e per un early-release solo-dev è un buon voto. Architettura senza cicli di import, CI tri-OS su ogni push, docs verificate da script in CI, debito tracciato onestamente in `TODO.md`. Zero segreti hardcoded, zero TLS bypass.

Il valore di questo audit non è la lista dei problemi — metà sono già nel tuo TODO — ma la distinzione tra **debito dichiarato** (quello in roadmap) e **debito implicito** (quello che nessuno ha annotato perché nessuno l'ha visto). Dei quattro punti impliciti originali, **dopo la riverifica ne resta aperto uno**: il circuit breaker morto (punto 1). Checksum updater e test del turn loop sono risolti e verificati; la regola 500 LoC è stata ripristinata sui file ma manca ancora il gate che ne impedisca l'erosione futura.

---

## Debito dichiarato (TODO.md) — l'audit conferma, non scopre

Punti già in `TODO.md` §4 e §8; qui solo conferme o correzioni di priorità:

- **Niente path jail su `readFile`/`editFile`/`find`** (`internal/agent/tools/read_file.go:53`, `edit_file.go:91`, `find.go:86`). Confermo che è il punto più serio: l'aggravante forse non pesata è che combinato con `fetchWeb` senza filtri diventa un canale di prompt injection → lettura di `~/.ssh` o scrittura fuori workspace. Nota positiva: la jail esiste già nel tuo codice (`replcomplete/path.go:198-205`, `plan/scan.go:25-27`) — è estrazione e riuso, non design nuovo.
- **Segreti in chiaro nel TOML** (vault, §4): per uso single-user con perms `0600`/`0700` già corretti in scrittura (`config_toml.go:563-583`), può aspettare il vault pianificato. Non anticiparlo.
- **Shell tool = comando reale** (§8): è il prodotto, non un bug. Solo da documentare come modello di fiducia.
- **Subagent, Anthropic thinking, Windows parity**: roadmap tua, fuori scope.

Unico riordino suggerito: la path jail (§8) merita di passare davanti al vault (§4) e a parecchie feature — piccola, e chiude il rischio più concreto.

---

## Debito implicito — quello che non potevi vedere a skimming

In ordine di priorità. I primi due li hai confermati come novità anche per te.

### 1. Il circuit breaker non fa niente (e sembrava esserci)

`internal/llm/httpresilience.go:175-189`: `CircuitRegistry.IsOpen` esiste, è testato, e **non è mai chiamato in produzione**; `ErrCircuitOpen` (`:23`) non è mai restituito. Con il provider down, ogni turno rifà fino a 10 retry con backoff — minuti di attesa invece di fail-fast. È il caso da manuale di codice generato: l'impalcatura completa del pattern (registry, `Trip`, `Reset`, test inclusi) senza l'ultimo filo collegato al punto d'uso, `runWithRetry` (`resilient_backend.go:50-88`). Compila, i test passano, a lettura veloce sembra finito. O si collega `IsOpen`, o si rimuove il registry: codice che simula una protezione è peggio di nessuna protezione.

### 2. L'erosione della regola 500 LoC non è controllata — ⚠️ SINTOMI RISOLTI, MANCA IL GATE

**Verificato post-fix (seconda passata):** tutti i file fuori soglia sono rientrati — `internal/llm/images/token.go` (598 righe) split nel sotto-package `images/token/` (`core.go`, `repl.go`, `mime.go`, `user.go`); `internal/llm/stream.go` scomposto (sotto-package `stream/`, `streamio/`, `stream_api.go`); `test/tool_display_checkpoint_test.go` (727) split in tre file da 161, 275 e 375 righe (`_checkpoint`, `_wrap`, `_format`), con divisione logica sensata. **Resta aperta solo la causa:** nessun gate automatico — niente `check_loc.go` in `scripts/`, nessuno step in CI o Makefile (`loc_chart` è solo visualizzazione). Finché il limite vive solo nelle regole e non in CI, il prossimo file generato sopra soglia passerà di nuovo inosservato.

*(Finding originale: nessun gate verificava la regola; `token.go` 598 righe, test cresciuto da 388 a 727 senza segnalazione; presentazione in accumulo dentro `internal/tooling`.)*

### 3. L'updater installa binari senza verificarli — ✅ RISOLTO

**Verificato post-fix:** la release genera `checksums.txt` (`release.yml:134-141`, `sha256sum` su tutti gli asset); l'updater verifica prima del rename (`verifyReleaseAsset` in `internal/updater/checksum.go:52-83`, chiamato da `install.go:144`); gli script di restart verificano anch'essi, sia bash (`restart.go:163-181`) che PowerShell (`restart.go:251-266`); fallback con warning per release vecchie senza checksum, come suggerito. Coperto da test (`test/updater_test.go:79` `TestInstall_verifiesChecksum`, `:122` `TestInstall_checksumMismatch` — asserisce il rifiuto su mismatch). Implementazione completa su tutti i percorsi individuati.

*(Finding originale: `install.go` scaricava da GitHub Releases con solo check dello status 200, nessun checksum né firma su nessun percorso — updater, script restart, install.sh. Auto-update opt-in, default off.)*

### 4. Il cuore del prodotto non ha rete di sicurezza — ✅ RISOLTO (test + race)

**Verificato post-fix:** esiste `test/turn_loop_test.go` con un backend scriptato (`turnScriptBackend`) e proprio i 5 scenari proposti: risposta semplice, round-trip tool con verifica dei messaggi persistiti, interrupt durante tool (con hook `SetExecToolHookForTest` e assert sul risultato sintetico), errore di stream, auto-compaction. I test asseriscono comportamento (ruoli, contenuti, conteggio chiamate al backend), non solo `err == nil`. In CI `go test -race` gira su Linux (`release.yml:39-43`). Il runtime espone hook di test dedicati (`NewTestRuntime`, `RunAgentTurnsForTest`) coerenti con la convenzione `test/` esterna.

**Andato oltre il piano:** il monolite non c'è più — `runAgentTurns` è stato estratto nel package dedicato `internal/agent/runtime/turnloop/` (`loop.go`, `host.go`, `interrupt.go`, `hook.go`) dietro un'interfaccia `Host` implementata dal bridge (`runtime/bridge.go:18-22`): era la voce "scomposizione" prevista come passo successivo, fatta insieme. Anche i `_ = persistSession` sono quasi tutti spariti: esiste `persistSessionOrLog` (`core.go:389-393`) e `turns.go` ha un solo persist, con errore gestito (`turns.go:113`). Residui minori di `_ =`: `legacy.go:146`, `repl_run.go:90,115`.

*(Finding originale: turn loop ~281 righe testato solo in `ResolveTurnInvocations`, CI senza `-race`, errori di persistenza zittiti in 5 punti.)*

### ~~5. Retry a metà stream duplica l'output~~ — riclassificato: scelta di design

`internal/llm/resilient_backend.go:98-105`: il retry riparte sempre dall'inizio del messaggio. **Non è un bug**: è la stessa politica fail-closed di Anthropic — mai far continuare a un LLM un messaggio già iniziato, perché la ripresa a metà è superficie di jailbreak. Coerente con il rifiuto dei chunk SSE incoerenti (`stream.go:195-197`). Resta solo una nota UX opzionale: a terminale il testo scartato e quello nuovo si susseguono senza separatore — un marcatore visivo ("riprovo da capo") renderebbe la scelta leggibile all'utente senza toccare la politica.

---

## Cose minori (degne di nota, non di urgenza)

- **SSRF su `fetchWeb`** (`fetch_web.go:65-71`): nessun blocco IP privati/loopback, redirect seguiti; `webSearch` searxng accetta override `baseURL` per-call (`web_search.go:78-81`). Teorico per uso personale; diventa bloccante se la Web UI in roadmap si concretizza.
- **Niente read-timeout sugli stream** (`httpresilience.go:218-221`): stream stallato = attesa infinita.
- **Sessioni JSON corrotte saltate in silenzio** (`chatstore/store.go:278-284`).
- **`ErrRestartSolomon` definito tre volte** (`slash/dispatch.go:15`, `runtime/restart.go:5`, `commands/update.go:11`).
- **`make test` richiede Node** anche per soli test Go (`Makefile:87-88`).
- **Performance** (stream `+=` O(n²), full-redraw REPL): nessuna evidenza di lentezza percepita — non toccare senza profilo. (Nota: `stream.go` è stato nel frattempo scomposto in `internal/llm/stream/` — riferimenti di riga originali non più validi, da ricontrollare se si interviene.)

Dimensioni in salute: dipendenze poche e aggiornate; documentazione accurata e verificata in CI; DevEx Windows curata (raro); nessun ciclo di import.

---

## Punti di forza da non rompere

CI tri-OS con vet+test+doc-check su ogni push; persistenza chat atomica (tmp+rename); fail-closed sugli stream SSE (`internal/llm/stream/completion.go:47,162`); sandbox WASM testata end-to-end con crash recovery; skill install argv-only con reject di metacaratteri; il pattern `toolenv`/`Deps` che tiene il grafo aciclico. E `TODO.md` stesso: pochi progetti solo-dev hanno un registro del debito così onesto.

---

## Strategia: gate automatici + la tua revisione, nell'ordine giusto

La revisione completa post-TODO che hai in mente resta una scelta tua e legittima — la capacità di macinare grandi quantità di LoC in review non è in discussione. Il punto è un altro: **i gate automatici prima della revisione la rendono più redditizia**, qualunque ne sia l'ampiezza. Tre ragioni:

1. I gate (lint, `-race`, check LoC) puliscono da soli la classe di problemi *meccanici* — errori zittiti, codice morto, file fuori soglia — così la tua review si concentra su ciò che solo un umano vede: logica, design, coerenza con l'intento.
2. I gate coprono anche **tutto il codice futuro**, incluso quello che verrà generato per i TODO rimanenti; una review manuale fotografa solo il presente, e farla prima della fine dei TODO significa rifarla.
3. Per chi sta ancora scoprendo Go, i messaggi di `staticcheck`/`errcheck` durante la review fanno da tutor sugli idiomi — rendono la review stessa più formativa.

In pratica: gate adesso, TODO con la rete sotto, e la revisione totale — se e quando deciderai di farla — su un codebase già sgrossato.

---

## Piano pratico (per una persona)

Stime: S < 2h, M = mezza giornata, L = 1-2 giorni.

**Subito (una sera, tutti S):**

1. ~~`go test -race ./...` in CI~~ ✅ fatto (Linux, `release.yml:39-43`).
2. golangci-lint con config minima (`errcheck`, `govet`, `staticcheck`, `ineffassign`); prima run rumorosa → eventualmente baseline con `--new-from-rev`. **Ancora aperto.**
3. Script `check_loc.go` in CI per la regola 500 (modello: `check_doc_paths.go`). **Ancora aperto — è il pezzo mancante del punto 2 del debito implicito.**
4. ~~Loggare gli errori `persistSession` ignorati~~ ✅ fatto (`persistSessionOrLog`, `core.go:389`); residui `_ =` in `legacy.go:146`, `repl_run.go:90,115`.
5. Segnalare le sessioni corrotte in `loadAllSessions`; unificare `ErrRestartSolomon`. **Da verificare/aperto.**

**Primo blocco — sicurezza reale:**

6. ~~Checksum nell'updater~~ ✅ fatto e testato (`checksum.go`, `release.yml:134-141`, `updater_test.go:79,122`).
7. **Path jail sui tool nativi** (M): estrarre la logica da `replcomplete/path.go:198-205` in un package condiviso, applicarla a read/edit/find, opt-out esplicito in config. Gotcha Windows: case-insensitive e drive letter. **Ora è il task aperto più importante.**

**Secondo blocco — rete di sicurezza per la roadmap:**

8. ~~Backend LLM fake + test del turn loop~~ ✅ fatto (`test/turn_loop_test.go`, 5 scenari), incluso lo step successivo non richiesto: estrazione del loop in `runtime/turnloop/`.
9. **Circuit breaker + read-timeout** (M): collegare `IsOpen` nel retry path e aggiungere timeout di inattività tra chunk. **Aperto — ultimo punto del debito implicito.** (Il restart-da-capo del retry resta com'è: design intenzionale anti-jailbreak.)

**Quando capita (S/M):** filtro SSRF su fetchWeb, target `make test-go` senza Node, escape HTML nel callback OAuth (`oauth_server.go:58-64`). (~~Limare il test display sotto 500~~ ✅ fatto: split in tre file da 161/275/375.)

**Esplicitamente rimandato:** vault (resta §4, i perms 0600 bastano per ora), refactoring del god object `Runtime` (`core.go:40-95` — costo alto, payoff basso finché sei solo e il flusso è leggibile), mitigazioni prompt-injection "strutturali" (problema aperto nel settore), ottimizzazioni performance senza profilo.

---

## Domande aperte

1. Il modello d'uso resta "io, sulla mia macchina"? Se sì il piano basta; se la Web UI si avvicina, SSRF e jail diventano bloccanti prima di esporla.
2. Jail rigida con opt-out in config, o conferma interattiva per operazioni fuori root? Cambia la UX.
3. Il legacy XML tool calling è strategico o in via di deprecazione? Decide quanto investire nei suoi test.
4. Da verificare a mano (10 min): i tool MCP in mode agent/build funzionano davvero? `toolParams` li espone (`mcp.go:72-74`) ma `modeAllowed` sembrerebbe bloccarli in `exec.go:154-155` — o c'è un bypass non trovato, o è una difesa accidentale.

---

*Review più leggera su: sidecar TypeScript `integrations/cursor/`, interni di `replcomplete`/`multiline`, script PowerShell. Massima profondità su: turn loop, llm, tools, sandbox, updater, auth.*
