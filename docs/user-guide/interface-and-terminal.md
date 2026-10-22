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
| Visualizzare deep research | GUI | Condivisa | Tab Deep research nella Home; `/research` e gli strumenti `deepResearch` / `researchStatus` nel terminale |
| Modificare rules, prompt, skills e subagents | GUI | Condivisa | Customization; `/rules`, `/add`, `/remove`, `/instructions` |
| Configurare MCP | Prevista | Terminale | `~/.solomon/mcp.json`, `/mcp` |
| Cercare nella documentazione Solomon | GUI | Condivisa | Sezione Docs in `/settings`; `/docs <query>` e `docsRetrieval` |
| Impostazioni di chat, modelli e documentazione | GUI | Condivisa | `/settings`; sezione `[gui]` |
| Esportare una chat in Markdown | Prevista | Terminale | `/export` |
| Configurare retry e timeout API | Prevista | Terminale | `config.toml`, sezione `[api_resilience]` |
| Aggiornare Solomon | GUI | Condivisa | Pulsante Update nel footer del pannello delle chat; `/update`, `/upgrade`, `/autoupdate` |

## GUI

La GUI è pensata per le operazioni visuali e contestuali:

- progetti, chat, file e worktree correnti;
- selezione del modello e del reasoning durante la composizione;
- terminale integrato;
- cataloghi di customization;
- ricerca e lettura visuale della documentazione;
- elenco dei job di Deep research e apertura dei report HTML;
- impostazioni che non richiedono di conoscere TOML, percorsi locali o comandi.

Le impostazioni si aprono dal side panel all'indirizzo `/settings`. Chat, modelli e documentazione restano sezioni di quella stessa pagina, senza un indirizzo separato. Il pulsante Back torna alla home su `/`.

Le chat persistite vengono lette e aggiornate dal daemon locale: la selezione nella sidebar apre la sessione Solomon esistente, il primo invio in un nuovo progetto crea la sessione e il turno viene trasmesso alla GUI con eventi incrementali. Se la GUI viene ricaricata durante un turno, il daemon continua l'esecuzione e la GUI si riaggancia alla sessione tramite replay degli eventi.

La sezione Docs, dentro `/settings`, carica i file Markdown inclusi nella cartella `docs/` e li rende navigabili e leggibili senza uscire dall'applicazione.

Nell'app desktop i link web e gli indirizzi email si aprono con l'applicazione predefinita del sistema. Puoi selezionare e copiare il testo della chat, incluso il ragionamento, senza richiuderlo durante la selezione. Il menu del tasto destro permette di copiare il testo selezionato e incollare nei campi di input; restano disponibili le scorciatoie standard e i pulsanti per copiare messaggi interi e blocchi di codice.

Quando è disponibile una nuova release, nel footer del pannello delle chat, tra il nome utente e le impostazioni, compare il pulsante con l'icona del razzo. Il tooltip **Download update** indica che il primo clic scarica la release e ne verifica l'integrità mantenendo aperta la GUI. Quando il download è pronto, il tooltip diventa **Restart to update**: il secondo clic installa la release e riavvia il daemon e i client desktop; la GUI web si riconnette e si ricarica quando il daemon torna disponibile. Il riavvio interrompe i turni e i comandi di terminale in corso, quindi avvialo quando sei pronto. Gli errori vengono mostrati sopra il pulsante e consentono di riprovare.

Il pulsante della GUI compare solo quando la release pubblicata supera la versione base locale: una build `v2026.1011.2-dev-…` non segnala la release `v2026.1011.2` come aggiornamento. Una release successiva resta installabile anche sopra una build locale modificata. `/upgrade` e `solomon upgrade` consentono inoltre di sostituire esplicitamente una build `dev` o `dirty` con la release ufficiale della stessa versione. I sorgenti locali vengono conservati. L'aggiornamento automatico mantiene invece la protezione delle build modificate, più recenti o non verificabili. Gli aggiornamenti conservano la modalità del server e la directory della GUI in modalità dev. Ricompilare e installare dal repository produce una build locale con suffisso `dev`, commit e indicatore `dirty` quando necessario. Make sceglie il tag di base più recente tra i tag locali e la versione del binario installato; per le installazioni consulta anche l'ultima release pubblicata. Se la rete non è disponibile, usa le informazioni locali. Una hot-install non torna quindi a un vecchio tag antenato del checkout dopo aver installato una release più recente. Il commit mostrato continua a identificare i sorgenti compilati; la build locale non viene presentata come una release ufficiale.

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
- usare gli strumenti agent per shell, file, ricerca web, documentazione e deep research.

Deep research mantiene un solo job store per progetto. La GUI legge i job dal
daemon locale e apre il report HTML; l'avvio, la cancellazione e la ripresa
restano disponibili tramite gli strumenti chat o `/research`.

Per la configurazione persistente vedere [Configuration](configuration.md). Per i comandi disponibili vedere [Usage and commands](usage-and-commands.md).

## Regola pratica

Se un'operazione riguarda il contesto visibile del progetto, cercarla prima nella GUI. Se riguarda credenziali, automazione, configurazione globale o diagnostica, usare il terminale. Quando una funzione esiste soltanto nel terminale, la GUI dovrebbe mostrarla come non ancora disponibile invece di lasciare un controllo ambiguo.

## See also

- [Configuration](configuration.md)
- [Usage and commands](usage-and-commands.md)
- [Data layout](data-layout.md)

### Riepilogo delle modifiche nella chat e nel terminale

In **Settings → Chat → File changes** puoi attivare il riepilogo
delle modifiche; nel terminale `/changes` alterna la stessa preferenza persistente
(`show_file_changes` nel file di configurazione). È attivo per impostazione predefinita.
Le impostazioni della GUI vengono lette dai nuovi processi CLI; `/changes`
aggiorna direttamente il terminale corrente.

Dopo i turni che modificano file, la GUI mostra un riquadro con cartelle
espandibili e un pulsante Open diff per i contributi della chat. Nel terminale
Solomon mostra cartelle e file senza sezioni collassate, con aggiunte in verde e rimozioni in rosso. Il riepilogo accumula
solo le modifiche degli strumenti `editFile` e `shell` di quella chat, inclusi
i richiami tramite `orchestrate`, e viene salvato con la chat. Le scritture
Solomon sullo stesso progetto vengono serializzate per separare le chat.
La scansione generale esclude i file ignorati da Git; i percorsi esplicitamente
indicati agli strumenti vengono tracciati anche se ignorati o esterni al progetto.
I file binari sono indicati separatamente. Le cartelle non accessibili non
interrompono il tracciamento. Per i comandi shell vengono riconosciuti i percorsi
letterali, incluse le espansioni delle variabili di ambiente; i percorsi calcolati
dinamicamente fuori dal progetto non sono sempre individuabili. Quando il
progetto è la home o la radice del filesystem, la scansione generale resta
al primo livello; i percorsi indicati al comando vengono comunque controllati.

Un commit eseguito nella chat (anche tramite `!git commit`) rimuove dal suo
riepilogo le modifiche committate. Un commit di un'altra chat non azzera questo
riepilogo. I commit parziali mantengono i contributi ancora da committare.
Le scritture effettuate da programmi esterni durante un comando shell non sono
attribuibili con certezza: il tracciamento delimita le operazioni degli strumenti
Solomon e non intercetta le scritture dei singoli processi del sistema operativo.
