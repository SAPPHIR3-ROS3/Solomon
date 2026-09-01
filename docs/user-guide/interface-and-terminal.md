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
| Mostrare reasoning e statistiche | GUI | Terminale | Transcript delle chat persistite; `/thinking`, `/stats` |
| Esplorare file del progetto | GUI | Condivisa | File explorer; `find`, `readFile`, `shell` |
| Cambiare branch o worktree | GUI | Condivisa | Controlli della Home; comandi `git` nel terminale |
| Aprire un terminale del progetto | GUI | Terminale | Terminal panel; PTY supervisionato dal daemon |
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

Le chat persistite vengono lette e aggiornate dal daemon locale: la selezione nella sidebar apre la sessione Solomon esistente, il primo invio in un nuovo progetto crea la sessione e il turno viene trasmesso alla GUI con eventi incrementali. Se la GUI viene ricaricata durante un turno, il daemon continua l'esecuzione e la GUI si riaggancia alla sessione tramite replay degli eventi.

La sezione Docs carica i file Markdown inclusi nella cartella `docs/` e li rende navigabili e leggibili senza uscire dall'applicazione.

## Transcript osservabile nella GUI

La GUI usa una superficie di transcript condivisa per le chat reali. Il renderer è in [`gui/src/chat/ChatView.tsx`](../../gui/src/chat/ChatView.tsx) e riceve i messaggi e gli eventi streaming dal client collegato al daemon.

La superficie mostra:

- reasoning collassabile, con il relativo `thought for`;
- statistiche del turno dell'assistente nel popup informativo;
- tool call con stato, icona, intent, comando e risultato;
- un checkpoint dedicato a ogni tool call, oltre ai checkpoint dei messaggi;
- anteprima della prima riga del risultato, con `...` cliccabile per espandere l'output completo;
- catena di tool call inizialmente contratta per singola card;
- controllo `Collapse tool calls` che sostituisce la catena con una sola cella `Show N tool calls`; i checkpoint contratti mostrano `[#start]...[#end]`, oppure `[#start][#end]` quando le tool call sono esattamente due.

### Convenzioni delle tool card

La riga di intent resta sempre separata dalla resa del tool. Aprendo una card, il contenuto segue la sintassi del terminale e mantiene i parametri leggibili:

- `shell`, `readFile`, `listDir` e `tree` mostrano il comando o il percorso sulla riga `Tool:`;
- `find` mostra la modalità e i parametri indentati, con un risultato iniziale a conteggio che può essere aperto per vedere gli elementi;
- `editFile` e `editPlan` mostrano rename, delete o diff in blocchi old/new espandibili;
- i plan tool mostrano i parametri sulla seconda riga; `todoList` parte dal riepilogo dei todo e apre la lista con quelli completati per primi;
- `deletePlan` mostra il nome del piano in rosso e non mostra un risultato aggiuntivo se l’operazione riesce;
- `fetchWeb` mostra URL e `timeout` su righe separate e nasconde il risultato in caso di successo;
- `webSearch` mostra la query sulla riga `Tool:`, i parametri indentati e gli oggetti `extras` come JSON formattato;
- durante un tool in esecuzione, il pallino animato resta sulla linea verticale della catena, insieme agli altri indicatori di stato.

I risultati che contengono una lista o un output lungo usano una preview cliccabile; l'espansione avviene direttamente sulla card, senza pulsanti aggiuntivi visibili.

Le etichette e i controlli dell'interfaccia sono in inglese anche quando il contenuto della chat è in italiano. Le chat dei progetti seguono il percorso persistito del daemon e condividono lo stesso transcript osservabile.

## Terminale

Il terminale resta la superficie completa per configurazione, automazione e diagnostica. La REPL interattiva (`solomon`, oppure `solomon tui`) è un client del PTY supervisionato dal daemon; `solomon exec` resta invece un comando headless diretto, adatto alle pipeline. In particolare il terminale consente di:

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
