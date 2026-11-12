# TUI 2.0

## Obiettivo
Ricreare la TUI con Bubble Tea, mantenendo aspetto e funzionalità attuali senza riscrivere l’harness o la GUI.

## Necessità
La crescita di Solomon richiede stato e rendering centralizzati: evitare scritture concorrenti, sovrapposizioni e regressioni; verificare il flickering, non presumere che il framework lo elimini.

## Da fare
- [ ] Approvare versione e dipendenze Bubble Tea; definire scrollback nativo o schermata alternativa.
- [ ] Separare eventi dell’harness e presentazione, con un solo proprietario del terminale.
- [ ] Ricreare composer, cronologia, completamento, incolla, allegati e scorciatoie.
- [ ] Migrare transcript, streaming, reasoning, tool, subagent e checkpoint.
- [ ] Coprire menu, selettori, onboarding, configurazione, notifiche, shell e /btw.
- [ ] Verificare resize, Unicode, cancellazione, interruzioni, EOF e ripristino terminale su Linux, macOS e Windows, inclusi PowerShell 5/7.
- [ ] Confrontare parità visiva e funzionale; rimuovere la vecchia TUI solo dopo le regressioni.

## Personalizzazione live permanente

La TUI 2.0 deve mantenere un nucleo di personalizzazione indipendente dai prototipi, nel modulo locale `tui/`. Palette, misure, bordi, testi e shortcut per azioni nominate si ricaricano senza compilare e senza perdere lo stato della sessione. La configurazione risiede in `SOLOMON_HOME/customization/tui/interface.json`, con override esplicito `SOLOMON_TUI_CONFIG`; non modifica provider, credenziali o configurazione dell’harness.

La modalità di sviluppo compila in background solo quando cambia il codice Go e trasferisce il terminale al nuovo frontend dopo il salvataggio atomico dello stato UI. Build fallite mantengono il frontend precedente. La component map usa gli stessi renderer della TUI. Questa infrastruttura è già collegata al laboratorio Bubble Tea; la TUI readline resta invariata fino alla migrazione del frontend, senza presumere parità già raggiunta.
