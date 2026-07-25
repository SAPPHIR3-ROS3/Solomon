# Solomon GUI — Context

Solomon avrà un **server locale per utente**, avviato e gestito in background con `solomon server`, indipendente dalla cwd e dal terminale chiamante. Il server ospita la GUI, espone l’API localhost e supervisiona processi worker Solomon separati per le interazioni effettive con runtime e macchina; lo stato dei workspace resta esplicito nelle richieste, non globale nel server.

`solomon web` apre la GUI browser e `solomon desktop` l’app Wails, assicurandosi prima che il server sia disponibile. In uso normale la GUI installata vive sotto `~/.solomon/gui/default/` e `~/.solomon/gui/user/`. In sviluppo, `solomon server start dev <gui-directory>` usa invece la directory GUI indicata, avvia Vite come helper per hot reload e non legge né modifica la GUI installata.

**Continuità:** durante lo sviluppo, aggiorna questo file quando un termine, un confine o una decisione diventa stabile. Mantienilo come contesto di lavoro locale per le sessioni future; la documentazione ufficiale sotto `docs/` viene aggiornata solo a funzionalità completata e verificata.

**Flusso Go:** quando una modifica coinvolge codice Go, il passaggio di applicazione e verifica locale concordato è `make install`. Non sostituirlo con `go run`, salvo istruzione esplicita dell'utente. Le modifiche esclusivamente sotto `gui/src/` non richiedono `make install` e vengono aggiornate da Vite/Wails in sviluppo.

**Server attivo:** non fermare, riavviare, reinstallare o altrimenti toccare il Solomon server già in esecuzione, salvo istruzione esplicita dell'utente. Per conoscere endpoint, porta, modalità e salute dell'istanza corrente, usare sempre `solomon server status`; non assumere né imporre una porta fissa.

**Modularità GUI:** `gui/src/` deve seguire la stessa filosofia modulare del resto della repository. Organizzare le responsabilità in moduli piccoli e coerenti (per esempio piattaforma, API client, stato, feature e componenti), evitando file monolitici o logica di infrastruttura dispersa nei componenti visivi. Browser e desktop condividono gli stessi moduli frontend; le differenze di piattaforma passano da adapter dedicati.

**Palette base:** la rampa foundation della GUI è Deep Royal Sapphire `#061C3B` (canvas), Royal Sapphire Night `#082A54` (surface), Deep Sapphire Blue `#0D3566` (surface raised) e Royal Azure `#174875` (border). L'identità cromatica è la triade Royal Sapphire, Crown Gold (`#FFC704`) e Crown Red (`#D40801`), con Sapphire Action (`#3B8FD1`) e Sapphire Highlight (`#86C9F2`) per stati interattivi. Gli stati semantici sono Brick Red (`#A83B3B`), Sea Green (`#237A52`) e Steel Blue (`#286C9F`), volutamente saturi e non pastello.

## Terms

| Term | Meaning |
|---|---|
| **Solomon server** | A user-scoped background process, manually managed through the Solomon CLI, that hosts the GUI, exposes the local API, and supervises worker processes. It is not tied to the current working directory or the terminal that started it. |
| **Server lifecycle** | The CLI starts, inspects, stops, restarts, and reads logs from the detached Solomon server, similarly to a local container manager. |
| **Web command** | `solomon web` ensures the Solomon server is available, then opens the web GUI URL in the default browser. |
| **Desktop command** | `solomon desktop` ensures the Solomon server is available, then opens the Wails desktop application. |
| **Wails desktop project** | `gui/desktop/` is the Wails project. Its configuration uses `gui/` as frontend root, so the shared React source remains under `gui/src/`. |
| **Wails development** | `wails dev` is run from `gui/desktop/`; it is a development-only tool and does not require `make install`. |
| **Client platform** | Shared GUI code detects both surface (`web` or Wails `desktop`) and OS (`macos`, `windows`, `linux`, or `unknown`) in `gui/src/platform.ts`. Wails asks its runtime for the OS; browser mode uses the user agent fallback. Title-bar controls that occupy the left edge keep their web position on the browser, while the macOS Wails surface reserves the traffic-light area at the same vertical alignment. |
| **GUI themes** | `~/.solomon/gui/themes/` is the mandatory common directory for installed themes; it is separate from both `default/` and `user/`. Solomon is dark-only: `gui/src/theme/` persists and applies the `dark` theme through `data-theme` on `<html>`. GUI colors use semantic CSS tokens in `gui/src/theme/themes.css`; components use tokens rather than literal colors. Future themes must be dark variants and add a token set rather than branching component styles. |
| **Interactive logs** | `solomon server logs interactive` streams new server log entries until interrupted, while `solomon server logs` prints the available log output and exits. |
| **Terminal panel cwd (temporary)** | Until workspace context is available in the GUI, every new terminal-panel shell starts in the user home directory (`$HOME`). The client does not choose its spawn cwd. |
| **Thread folders (development)** | The sidebar reads registered workspace folders from `~/.solomon/projectsId.json`; it also inspects the associated `projects/<id>/chats/` directory for the thread count. The same read-only response reads `user_name` from `~/.solomon/config.toml` for the sidebar user area. Until the conversations API exists, the Vite development middleware serves this local data at `/__solomon/thread-folders`. |
| **Server health** | `GET /health` reports the running server’s version, PID, URL, start time, mode, runtime details, and current Vite/API/GUI/worker status. The base prototype currently reports the latter services as not configured. |
| **Local server URL** | The normal Solomon server binds to `http://localhost:8765` (with equivalent numeric bind `http://127.0.0.1:8765`); development mode selects a free loopback port. Wails development obtains the running server’s advertised local URL from server state through `make desktop-dev`, rather than assuming a port. |
| **Solomon worker** | A child `solomon` process started by the Solomon server for one effective interaction with the Solomon runtime. |
| **Workspace context** | The project context supplied explicitly by a request. The Solomon server does not maintain an implicit active workspace. |
| **GUI bundle root** | `~/.solomon/gui/`, the user-owned location for GUI files. It contains the `default/`, `user/`, and mandatory shared `themes/` directories. Its installation, update, and host mechanisms are still undecided. |
| **Default GUI** | The Solomon-provided GUI layer, intended to live under `~/.solomon/gui/default/`. |
| **User GUI** | The user-owned customization layer, intended to live under `~/.solomon/gui/user/`. Its override semantics are still undecided. |
| **GUI development mode** | A local mode in which the Solomon server uses the `gui/` directory of a repository checkout and coordinates its frontend dev server. It does not read or modify `~/.solomon/gui/`; source changes receive native frontend hot-module reload. |
| **Dev mode command** | `solomon server start dev <gui-directory>` starts development mode from the explicitly supplied GUI-project directory; its resolved absolute path is retained in server state. |
| **Dev GUI directory** | The explicit directory passed to development mode. It is the frontend project root and contains its source, package manifest, and frontend tooling configuration. |
| **Vite development helper** | The frontend development process owned by the Solomon server in dev mode. It compiles the GUI source and provides hot-module reload; it is not part of the installed server runtime. |
| **Dev lifecycle** | Starting the Solomon server in dev mode starts its Vite helper; stopping or restarting the server also stops that helper. The CLI may exit while both remain running. |
| **Installed GUI reload** | A full client reload triggered when files under `~/.solomon/gui/` change; it supports direct user edits but does not compile source code. |
