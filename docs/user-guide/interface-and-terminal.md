# Interfaccia e terminale

Solomon espone alcune funzioni nella GUI e altre nel terminale. Questa pagina indica dove trovare ogni operazione, evitando di confondere una funzione già disponibile con una ancora prevista.

## Legenda

| Indicazione | Significato |
| --- | --- |
| GUI | Disponibile nell'interfaccia grafica web o desktop |
| Terminale | Disponibile nella REPL o nei comandi `solomon` |
| Condivisa | Disponibile su entrambe le superfici, con un flusso diverso |
| Prevista | Parte del modello del progetto, ma non ancora esposta in quella superficie |

## Mappa delle funzioni

| Funzione | GUI | Terminale | Dove cercarla |
| --- | --- | --- | --- |
| Aprire un progetto e vedere le chat | GUI | Condivisa | Side panel; `/new`, `/resume` |
| Creare una chat | GUI | Condivisa | New chat; `/new` |
| Selezionare provider e modello | GUI | Condivisa | Model picker; `/models` |
| Collegare un provider | Prevista | Terminale | `/connect`, `/onboard` o `config.toml` |
| Cambiare reasoning | GUI | Condivisa | Controllo reasoning nella Home; `/reasoning` |
| Attivare fast mode | Prevista | Terminale | `/fast` |
| Cambiare lingua delle risposte | Prevista | Terminale | `/language` |
| Mostrare reasoning e statistiche | Prevista | Terminale | `/thinking`, `/stats` |
| Esplorare file del progetto | GUI | Condivisa | File explorer; `find`, `readFile`, `shell` |
| Cambiare branch o worktree | GUI | Condivisa | Controlli della Home; comandi `git` nel terminale |
| Aprire un terminale del progetto | GUI | Terminale | Terminal panel; shell locale |
| Visualizzare deep research | GUI | Prevista | Research panel; gli strumenti `webSearch` e `fetchWeb` sono disponibili nel terminale agente |
| Modificare rules, prompt, skills e subagents | GUI | Condivisa | Customization; `/rules`, `/add`, `/remove`, `/instructions` |
| Configurare MCP | Prevista | Terminale | `~/.solomon/mcp.json`, `/mcp` |
| Cercare nella documentazione Solomon | GUI | Condivisa | Sezione Docs; `/docs <query>` e `docsRetrieval` |
| Esportare una chat in Markdown | Prevista | Terminale | `/export` |
| Configurare retry e timeout API | Prevista | Terminale | `config.toml`, sezione `[api_resilience]` |
| Aggiornare Solomon | Prevista | Terminale | `/update`, `/upgrade`, `/autoupdate` |

## GUI

La GUI è pensata per le operazioni visuali e contestuali:

- progetti, chat, file e worktree correnti;
- selezione del modello e del reasoning durante la composizione;
- terminale integrato;
- cataloghi di customization;
- ricerca e lettura visuale della documentazione;
- impostazioni che non richiedono di conoscere TOML, percorsi locali o comandi.

La sezione Docs carica i file Markdown inclusi nella cartella `docs/` e li rende navigabili e leggibili senza uscire dall'applicazione.

## Terminale

Il terminale resta la superficie completa per configurazione, automazione e diagnostica. In particolare consente di:

- collegare provider e autenticazioni;
- modificare configurazioni avanzate;
- usare slash command e modalità `exec`/`temp exec`;
- installare e gestire skills, rules, MCP e subagents;
- esportare conversazioni e controllare aggiornamenti;
- usare gli strumenti agent per shell, file, ricerca web e documentazione.

Per la configurazione persistente vedere [Configuration](configuration.md). Per i comandi disponibili vedere [Usage and commands](usage-and-commands.md).

## Regola pratica

Se un'operazione riguarda il contesto visibile del progetto, cercarla prima nella GUI. Se riguarda credenziali, automazione, configurazione globale o diagnostica, usare il terminale. Quando una funzione esiste soltanto nel terminale, la GUI dovrebbe mostrarla come non ancora disponibile invece di lasciare un controllo ambiguo.

## See also

- [Configuration](configuration.md)
- [Usage and commands](usage-and-commands.md)
- [Data layout](data-layout.md)
