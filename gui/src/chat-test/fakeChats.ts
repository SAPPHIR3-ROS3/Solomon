export type FakeChatMessage = {
  checkpointBranch?: string;
  checkpointSeq?: number;
  createdAt?: number;
  id: string;
  role: "assistant" | "user";
  content: string;
};

export type FakeChat = {
  createdAt?: number;
  id: string;
  isNewTestChat?: boolean;
  messages: FakeChatMessage[];
  title: string;
  workspaceID?: string;
  worktree?: string;
};

// Deliberately UI-only fixtures. They are never written to ~/.solomon and let
// us iterate on the conversation surface before wiring the worker/API.
export const initialFakeChats: FakeChat[] = [
  {
    id: "test-product-brief",
    title: "Product brief",
    worktree: "local",
    messages: [
      { id: "brief-user", role: "user", content: "Prepara una checklist completa per il lancio della nuova esperienza chat, includendo obiettivi, utenti coinvolti, flusso critico, responsabilità e criteri di successo." },
      { id: "brief-assistant", role: "assistant", content: "## Checklist di rilascio\n\nPartirei da cinque aree, con un responsabile esplicito per ciascuna:\n\n- **Obiettivo:** ridurre il tempo necessario per arrivare al primo messaggio utile.\n- **Utenti pilota:** team interno e un piccolo gruppo di utenti abituali.\n- **Flusso critico:** apertura → scelta del contesto → primo messaggio → risposta → nuova richiesta.\n- **Responsabilità:** prodotto, engineering, supporto e una persona incaricata del go/no-go.\n- **Successo:** completamento del flusso, latenza e qualità percepita della risposta.\n\n> La checklist deve essere abbastanza concreta da poter essere usata durante il rollout, non solo in fase di pianificazione." },
      { id: "brief-user-2", role: "user", content: "Aggiungi anche una parte dedicata ai **rischi tecnici e operativi**, con un’indicazione concreta di come ridurli prima di esporre la funzione a tutti gli utenti." },
      { id: "brief-assistant-2", role: "assistant", content: "### Rischi e mitigazioni\n\n| Rischio | Segnale | Mitigazione |\n| --- | --- | --- |\n| Regressioni | test falliti o errori UI | smoke test automatico + verifica manuale |\n| Latenza | p95 sopra la baseline | limite di rollout e monitoraggio del primo token |\n| Stato perso | conversazione che riparte vuota | test di riapertura e controllo della persistenza |\n| Rollback incerto | nessun proprietario | runbook provato prima del go-live |\n\nPer ogni rischio assegnerei un proprietario, una soglia di allarme e una decisione già concordata. Se il segnale supera la soglia, il rollout si ferma: non si decide durante l’incidente." },
      { id: "brief-user-3", role: "user", content: "Come organizzeresti la fase di verifica in modo che sia ripetibile anche nei prossimi rilasci e non dipenda solo da controlli manuali fatti all’ultimo momento?" },
      { id: "brief-assistant-3", role: "assistant", content: "Userei una checklist versionata per ambiente e la farei girare in CI insieme a test automatici sulle API. Prima del rilascio aggiungerei uno smoke test manuale sul percorso principale, una verifica responsive e un controllo finale di log e metriche.\n\n```text\nverify: api → streaming → persistence → responsive → rollback\n```\n\nCosì ogni rilascio lascia una traccia confrontabile con il precedente." },
      { id: "brief-user-4", role: "user", content: "Quali metriche mostreresti al team durante il rollout e quali segnali useresti per decidere se continuare, rallentare o interrompere la distribuzione?" },
      { id: "brief-assistant-4", role: "assistant", content: "## Metriche di rollout\n\nSeparerei i segnali in tre livelli, così il team può capire rapidamente se il problema riguarda l’esperienza, l’affidabilità o la capacità del sistema.\n\n1. **Esperienza utente**\n   - tempo al primo token (`TTFT`);\n   - completamento del flusso principale;\n   - feedback qualitativo degli utenti pilota.\n2. **Affidabilità**\n   - success rate;\n   - errori per endpoint;\n   - interruzioni, timeout e retry.\n3. **Capacità e costi**\n   - token medi per richiesta;\n   - saturazione del modello;\n   - consumo rispetto alla previsione.\n\n| Segnale | Verde | Giallo | Rosso |\n| --- | ---: | ---: | ---: |\n| Success rate | ≥ 99% | 97–99% | < 97% |\n| Latenza p95 | < 2 s | 2–4 s | > 4 s |\n| Errori bloccanti | 0 | 1–2 | > 2 |\n\n> La baseline non è un numero decorativo: serve a capire se il nuovo flusso migliora davvero l’esperienza precedente.\n\n**Regola pratica:** continuare se i valori restano verdi, rallentare il rollout se compaiono segnali gialli ricorrenti, interromperlo davanti a un singolo segnale rosso che blocca il percorso principale." },
      { id: "brief-user-5", role: "user", content: "Serve anche una strategia per il rollback? Vorrei che fosse chiara abbastanza da poter essere eseguita rapidamente anche durante un incidente reale." },
      { id: "brief-assistant-5", role: "assistant", content: "## Piano di rollback\n\nIl rollback deve poter essere eseguito senza ricostruire il contesto durante l’incidente. Prima della finestra di rilascio preparerei un runbook breve, con un responsabile nominato e una versione stabile già verificata.\n\n- [x] identificare la versione stabile precedente;\n- [x] definire chi può fermare il rollout;\n- [ ] provare il ripristino in ambiente di staging;\n- [ ] verificare i dati dopo il rollback;\n- [ ] comunicare il ritorno alla versione precedente.\n\nIl percorso operativo dovrebbe essere simile a questo:\n\n```bash\nsolomon rollout pause --env production\nsolomon deploy --version <stable-version>\nsolomon healthcheck --wait 60\n```\n\nDopo il ripristino controllerei tre cose: **nuove richieste**, conversazioni già aperte e log di errore. Se il servizio risponde ma lo stato delle chat è incoerente, il rollback non è ancora concluso.\n\n> Un rollback provato prima vale più di una procedura perfetta ma mai eseguita." },
      { id: "brief-user-6", role: "user", content: "Chi dovrebbe approvare il rilascio e quali informazioni minime dovrebbe avere prima di dare il via libera definitivo?" },
      { id: "brief-assistant-6", role: "assistant", content: "### Approvazione del rilascio\n\nChiederei due approvazioni indipendenti: una dal prodotto e una dall’area tecnica. Non serve una riunione lunga, ma servono informazioni verificabili e una decisione esplicita.\n\n- **Referente prodotto**\n  - obiettivo del rilascio;\n  - utenti coinvolti;\n  - risultati della fase pilota;\n  - messaggio da condividere con gli utenti.\n- **Referente tecnico**\n  - stato dei test;\n  - rischi ancora aperti;\n  - metriche osservate;\n  - piano di rollback e responsabili operativi.\n\nPrima del via libera chiederei anche: *che cosa ci farebbe cambiare idea?* Se la risposta non è chiara, la decisione è prematura.\n\nLa registrazione minima potrebbe essere:\n\n```yaml\ndecision: go\nowners: [product, engineering]\nrollout: 10% -> 25% -> 50% -> 100%\nrollback_owner: on-call\n```\n\n**Criterio di chiusura:** entrambi approvano la stessa versione, la stessa finestra e la stessa soglia di interruzione." },
      { id: "brief-user-7", role: "user", content: "Perfetto, chiudiamo con i prossimi passi ordinati per priorità, indicando cosa deve essere fatto prima della finestra di rilascio e cosa invece può essere completato subito dopo." },
      { id: "brief-assistant-7", role: "assistant", content: "### Prossimi passi\n\n1. Assegnare i responsabili e fissare la finestra di rilascio.\n2. Completare smoke test e prova di rollback.\n3. Preparare monitoraggio e messaggio per gli utenti pilota.\n4. Durante il rollout, monitorare errori, latenza e feedback.\n5. Dopo il lancio, confrontare i dati con la baseline e registrare le decisioni.\n\n**Decisione attesa:** procedere per percentuali crescenti, con un punto di controllo esplicito a ogni fase." },
      { id: "brief-user-8", role: "user", content: "Aggiungi un controllo specifico per la qualità della risposta e per i casi in cui l’utente interrompe il flusso prima della fine." },
      { id: "brief-assistant-8", role: "assistant", content: "## Qualità e interruzioni\n\nVerificherei la qualità della risposta su tre dimensioni diverse:\n\n- **completezza:** la risposta copre la richiesta senza fermarsi a metà;\n- **coerenza:** mantiene il contesto e non contraddice i messaggi precedenti;\n- **recuperabilità:** l’utente può correggere, riprovare o riprendere il flusso.\n\n### Casi da simulare\n\n1. Risposta interrotta durante lo streaming.\n2. Timeout subito dopo il primo token.\n3. Retry dopo un errore temporaneo.\n4. Interruzione volontaria con il pulsante di stop.\n5. Chiusura e riapertura della conversazione.\n\nPer ogni caso registrerei **causa**, **punto dell’interruzione**, **stato mostrato all’utente** e **azione disponibile**.\n\n~~Controllo manuale alla fine e basta~~ non è sufficiente: il test deve verificare anche quello che succede nei secondi intermedi, quando la risposta è ancora in streaming." },
      { id: "brief-user-9", role: "user", content: "Come gestiresti invece i messaggi molto lunghi e le conversazioni che non entrano più nel contesto disponibile?" },
      { id: "brief-assistant-9", role: "assistant", content: "## Conversazioni oltre il contesto\n\nQuando una chat diventa molto lunga, non cercherei di nascondere il problema. Mostrerei uno stato esplicito e conserverei un riepilogo leggibile, mantenendo il punto di lettura dell’utente.\n\nLa strategia potrebbe avere quattro passaggi:\n\n1. **Misurare** il contesto usato e quello ancora disponibile.\n2. **Compattare** i turni più vecchi, separando decisioni, vincoli e dettagli ormai superati.\n3. **Conservare** il riepilogo come parte visibile della conversazione.\n4. **Riprendere** il lavoro indicando quali informazioni sono state riutilizzate.\n\nUn riepilogo strutturato potrebbe assomigliare a questo:\n\n```text\n[decisioni] usare Markdown nel transcript\n[vincoli] non modificare la grafica\n[aperto] collegare la chat al runtime reale\n```\n\n> Compattare non significa cancellare in silenzio: l’utente deve poter capire perché la risposta continua a essere coerente." },
      { id: "brief-user-10", role: "user", content: "Vorrei includere anche una verifica su immagini, file allegati e riferimenti a percorsi del progetto, perché sono elementi importanti per il lavoro quotidiano." },
      { id: "brief-assistant-10", role: "assistant", content: "## Allegati e riferimenti\n\nPer immagini, file e percorsi del progetto distinguerei sempre tra **contenuto disponibile**, **contenuto in caricamento** e **contenuto non accessibile**. La risposta non dovrebbe trattare questi tre casi nello stesso modo.\n\n| Caso | Cosa mostrare | Azione possibile |\n| --- | --- | --- |\n| File pronto | nome, tipo e dimensione | aprire o rimuovere |\n| Upload in corso | progresso e stato | annullare |\n| File mancante | errore comprensibile | ritentare o sostituire |\n| Percorso non valido | percorso ricevuto | correggere riferimento |\n\nAggiungerei test per:\n\n- immagini incollate e immagini troppo grandi;\n- file mancanti o duplicati;\n- percorsi relativi e assoluti;\n- riferimenti allo stesso documento in più messaggi;\n- allegati rimossi mentre la risposta è ancora in streaming.\n\nPer i riferimenti al codice, un link come [`gui/src/chat-test/fakeChats.ts`](../chat-test/fakeChats.ts) dovrebbe restare leggibile anche quando il messaggio viene riaperto. L’errore deve spiegare **che cosa manca** e **come risolverlo**, non limitarsi a dire “fallito”." },
      { id: "brief-user-11", role: "user", content: "Puoi chiudere con un riepilogo operativo che il team possa usare durante la riunione di go-live?" },
      { id: "brief-assistant-11", role: "assistant", content: "## Riepilogo operativo\n\n### Prima della finestra\n\n- [ ] confermare responsabili e orario;\n- [ ] validare test, metriche e baseline;\n- [ ] provare il rollback;\n- [ ] controllare streaming, allegati e persistenza;\n- [ ] preparare monitoraggio e comunicazioni.\n\n### Durante il rollout\n\nProcederei per percentuali crescenti:\n\n1. 10% degli utenti;\n2. controllo degli errori e della latenza;\n3. 25% se i segnali sono stabili;\n4. nuovo controllo prima del 50%;\n5. estensione completa solo dopo l’ultima decisione.\n\n> Go solo se il percorso principale funziona, il rollback è eseguibile e ogni segnale rosso ha un proprietario.\n\n**Decisione finale:** procedere a piccoli passi, registrando dopo ogni fase cosa è stato osservato, quale scelta è stata fatta e perché." },
    ],
  },
  {
    id: "test-tool-result",
    title: "Tool e risultati",
    messages: [
      { id: "tool-user", role: "user", content: "Quali elementi deve mostrare un turno dell’agente?" },
      { id: "tool-assistant", role: "assistant", content: "## Un turno osservabile\n\nUn turno dell’agente dovrebbe rendere comprensibile sia **che cosa sta facendo** sia **che cosa ha ottenuto**. Mostrerei questi elementi nell’ordine in cui accadono:\n\n1. messaggio dell’utente;\n2. reasoning o piano sintetico;\n3. attività dei tool;\n4. risultato del tool;\n5. risposta finale;\n6. errori e metriche del turno.\n\n| Elemento | Serve a | Esempio |\n| --- | --- | --- |\n| Reasoning | spiegare il prossimo passo | “Cerco il file di configurazione” |\n| Tool call | mostrare l’azione | `read_file(path)` |\n| Risultato | rendere verificabile l’azione | file trovato, 84 righe |\n| Stato | indicare come è finito il turno | completato o interrotto |\n\nUn risultato tecnico potrebbe essere rappresentato così:\n\n```json\n{\n  \"status\": \"completed\",\n  \"tool\": \"read_file\",\n  \"duration_ms\": 184,\n  \"items\": 3\n}\n```\n\n> La trasparenza non significa mostrare ogni dettaglio interno: significa lasciare abbastanza tracce perché l’utente possa capire e verificare il lavoro." },
    ],
  },
];

export function createNewFakeChat(workspaceID?: string, title = "New chat", createdAt = Date.now()): FakeChat {
  return {
    createdAt,
    id: `test-new-${createdAt}-${Math.random().toString(36).slice(2, 8)}`,
    isNewTestChat: true,
    messages: [],
    title,
    workspaceID,
    worktree: "local",
  };
}

// Browser-only equivalent of chatstore.NewPlaceholderChatID. Solomon uses
// the UTC RFC3339Nano timestamp as the temporary title/id and replaces colons
// so it can be used safely in a filename. Date exposes milliseconds in the
// browser, so preserve the same shape with zero-padded microseconds.
export function newPlaceholderChatID(now = new Date()): string {
  const [dateAndTime, milliseconds = "000"] = now.toISOString().slice(0, -1).split(".");
  return `newchat-${dateAndTime.replaceAll(":", "-")}.${milliseconds.padEnd(6, "0")}Z`;
}

export function fakeAssistantReply(text: string): FakeChatMessage {
  const quotedText = text.trim().split(/\r?\n/).map((line) => `> ${line || " "}`).join("\n");
  return {
    createdAt: Date.now(),
    id: `assistant-${Date.now()}`,
    role: "assistant",
    content: `## Risposta di test\n\nHo ricevuto la richiesta e l’ho aggiunta al turno simulato. Per rendere visibile il comportamento del transcript, qui sotto mostro sia l’interpretazione sia l’output prodotto.\n\n### Richiesta originale\n\n${quotedText}\n\n### Interpretazione\n\n- il messaggio è stato ricevuto correttamente;\n- il contenuto resta disponibile nel contesto del turno;\n- la risposta seguente è solo un esempio, non un risultato del runtime reale.\n\n### Output simulato\n\n~~~text\nparse request\ncreate assistant turn\nrender markdown\n~~~\n\n**Stato:** risposta simulata\n\n> Il collegamento al runtime Solomon arriverà nel prossimo passaggio.\n\nSe vuoi, puoi inviare un altro messaggio usando \`**grassetto**\`, una lista o un blocco di codice per verificare il rendering in tempo reale.`,
  };
}
