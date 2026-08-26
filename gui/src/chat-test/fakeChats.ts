export type FakeChatImage = {
  name: string;
  url: string;
};

export type FakeChatStats = {
  contextTokens: number;
  outputTokensPerSecond: number;
  promptTokensPerSecond: number;
  reasoningTokens: number;
  responseTokens: number;
  totalTokens: number;
  ttftSeconds: number;
  userTokens: number;
};

export type FakeChatToolResult = {
  count?: number;
  completed?: number;
  items?: string[];
  output?: string;
  summary?: string;
  status: "error" | "success";
  todoItems?: FakeChatTodoItem[];
  truncated?: boolean;
};

export type FakeChatTodoItem = {
  checked: boolean;
  text: string;
};

export type FakeChatToolParameter = {
  label: string;
  value: string;
};

export type FakeChatToolCall = {
  checkpointBranch?: string;
  checkpointSeq?: number;
  defaultOpen?: boolean;
  delete?: boolean;
  full?: boolean;
  id: string;
  input?: string;
  intent?: string;
  mode?: "files" | "text";
  name: string;
  newString?: string;
  oldString?: string;
  parameters?: FakeChatToolParameter[];
  result?: FakeChatToolResult;
  renameTo?: string;
  status?: "error" | "running" | "success";
};

export type FakeRetainedMessage = {
  content: string;
  images?: FakeChatImage[];
  role: "assistant" | "user";
};

export type FakeChatMessage = {
  checkpointBranch?: string;
  checkpointSeq?: number;
  createdAt?: number;
  id: string;
  images?: FakeChatImage[];
  kind?: "compaction";
  role: "assistant" | "user";
  reasoning?: string;
  stats?: FakeChatStats;
  status?: "interrupted";
  thoughtFor?: number;
  toolCalls?: FakeChatToolCall[];
  workedFor?: number;
  content: string;
  retainedMessages?: FakeRetainedMessage[];
  summary?: string;
};

export type FakeChat = {
  createdAt?: number;
  id: string;
  isNewTestChat?: boolean;
  messages: FakeChatMessage[];
  modeSwitchTarget?: "agent";
  title: string;
  workspaceID?: string;
  worktree?: string;
};

// Mirrors Solomon's built-in tool registry for the test transcript. The
// orchestration runner is intentionally omitted from this showcase.
const allAvailableFakeToolCalls = ([
  {
    id: "tool-shell-check",
    input: "go test ./gui/...",
    intent: "Verifico il renderer della chat prima di mostrare un turno osservabile.",
    name: "shell",
    result: { output: "?    solomon/gui    [no test files]\n✓ build completed", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-read-view",
    input: "gui/src/chat-test/TestChatView.tsx",
    intent: "Leggo il transcript per collegare attività, risultato e risposta.",
    name: "readFile",
    result: { output: "function ChatMessageTurn(...) {\n  // render reasoning, tools and response\n}", status: "success", truncated: true },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-find-view",
    input: "tool card intent",
    intent: "Cerco i riferimenti alle tool card nel progetto.",
    mode: "text",
    name: "find",
    parameters: [
      { label: "pattern", value: "tool card intent" },
      { label: "path", value: "gui/src/chat-test" },
    ],
    result: {
      count: 2,
      items: ["gui/src/chat-test/TestChatView.tsx: tool intent", "gui/src/chat-test/test-chat.css: tool card"],
      status: "success",
    },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-list-dir",
    input: "gui/src/chat-test",
    intent: "Elenco i file della chat di test.",
    name: "listDir",
    result: {
      count: 3,
      items: ["TestChatView.tsx", "fakeChats.ts", "test-chat.css"],
      status: "success",
    },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-tree",
    input: "gui/src/chat-test",
    intent: "Controllo la struttura della chat di test.",
    name: "tree",
    result: {
      output: "chat-test/\n├── TestChatView.tsx\n├── fakeChats.ts\n└── test-chat.css",
      status: "success",
      summary: "3 nodes",
    },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-edit-view",
    input: "old/path.ts",
    name: "editFile",
    renameTo: "new/path.ts",
    intent: "Rinomino il file del renderer.",
    result: { output: "renamed", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-edit-apply",
    input: "gui/src/chat-test/TestChatView.tsx",
    intent: "Aggiorno il renderer della tool card.",
    name: "editFile",
    oldString: "function ChatMessageTurn(...) {\n  return <article>...</article>\n}",
    newString: "function ChatMessageTurn({ message }) {\n  return <article className=\"chat-turn\">...</article>\n}",
    result: { output: "edited", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    delete: true,
    id: "tool-edit-delete",
    input: "gui/src/chat-test/obsolete.ts",
    intent: "Rimuovo il file non più utilizzato.",
    name: "editFile",
    result: { output: "deleted", status: "success" },
    status: "success",
  },
  {
    id: "tool-search-tools",
    input: "workspace file tools",
    intent: "Cerco nel catalogo gli strumenti disponibili per il workspace.",
    name: "searchTools",
    result: {
      count: 4,
      items: ["shell", "readFile", "editFile", "find"],
      output: "shell · readFile · editFile · find",
      status: "success",
    },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-search-skill",
    input: "frontend design",
    intent: "Cerco una skill adatta alla modifica dell’interfaccia.",
    name: "searchSkill",
    result: { count: 1, output: "frontend-design", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-load-skill",
    input: "frontend-design",
    intent: "Carico le istruzioni della skill selezionata.",
    name: "loadSkill",
    result: { output: "# Frontend Design\nDesign guidance loaded", status: "success", truncated: true },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-subagent",
    input: "review the chat transcript renderer",
    intent: "Delego una revisione mirata del renderer.",
    name: "subagent",
    result: { output: "review completed", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-switch-mode",
    input: "agent",
    intent: "Passo alla modalità agente per eseguire modifiche.",
    name: "switchMode",
    result: { output: "mode: agent", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-list-sub-agents",
    input: "configured roles",
    intent: "Controllo gli agenti secondari configurati.",
    name: "listSubAgents",
    result: { output: "no active subagents", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-docs-retrieval",
    input: "chat transcript tool calls",
    intent: "Cerco nella documentazione la convenzione del transcript.",
    name: "docsRetrieval",
    result: { output: "2 documentation snippets found", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-create-plan",
    input: "chat-tools",
    intent: "Creo un piano per censire gli strumenti della chat.",
    name: "createPlan",
    parameters: [{ label: "goal", value: "inventory the available tools" }],
    result: { output: "plan created: chat-tools", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-edit-plan",
    input: "chat-tools",
    intent: "Aggiorno il piano con l’inventario dei tool.",
    name: "editPlan",
    oldString: "## Tools\n- shell\n- readFile\n- editFile\n- find",
    newString: "## Tools\n- shell\n- readFile\n- editFile\n- find\n- createPlan",
    result: { output: "plan updated", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-build-plan",
    input: "chat-tools",
    intent: "Preparo il brief di implementazione dal piano.",
    name: "buildPlan",
    result: { output: "implementation brief ready", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-add-todo",
    input: "chat-tools",
    intent: "Aggiungo il controllo dell’inventario al piano.",
    name: "addTodo",
    parameters: [{ label: "todo", value: "Validate the complete tool inventory" }],
    result: { output: "todo added", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-todo-list",
    input: "chat-tools",
    intent: "Controllo le attività ancora aperte.",
    name: "todoList",
    result: {
      completed: 1,
      count: 2,
      status: "success",
      todoItems: [
        { checked: false, text: "Validate the complete tool inventory" },
        { checked: true, text: "Confirm the terminal tool order" },
      ],
    },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-check-todo",
    input: "abc123",
    intent: "Segno come completata l’attività verificata.",
    name: "checkTodo",
    result: { output: "todo checked", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-check-todo-failed",
    input: "def456",
    intent: "Provo a completare un’attività non più presente nel piano.",
    name: "checkTodo",
    result: { output: "sha not found", status: "error" },
    status: "error",
  },
  {
    defaultOpen: false,
    id: "tool-remove-todo",
    input: "abc123",
    intent: "Rimuovo l’attività già completata dal piano.",
    name: "removeTodo",
    result: { output: "todo removed", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-remove-todo-failed",
    input: "def456",
    intent: "Provo a rimuovere un’attività non più presente nel piano.",
    name: "removeTodo",
    result: { output: "sha not found", status: "error" },
    status: "error",
  },
  {
    defaultOpen: false,
    full: true,
    id: "tool-check-plan",
    input: "chat-tools",
    intent: "Controllo lo stato del piano prima di procedere.",
    name: "checkPlan",
    result: { output: "status: implementing", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-delete-plan",
    input: "chat-tools",
    intent: "Rimuovo il piano temporaneo non più necessario.",
    name: "deletePlan",
    result: { output: "plan deleted", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-fetch-web",
    input: "https://example.com/solomon-guide",
    intent: "Recupero il contenuto della guida indicata.",
    name: "fetchWeb",
    parameters: [{ label: "timeout", value: "30" }],
    result: { output: "# Solomon guide\nTool transcript reference", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-web-search",
    input: "tool card result UI",
    intent: "Cerco un esempio lasciando visibile la ricerca in corso.",
    name: "webSearch",
    parameters: [
      { label: "engine", value: "searxng" },
      { label: "maxResults", value: "5" },
      { label: "timeout", value: "45" },
      { label: "extras", value: "{\n  \"baseURL\": \"https://search.example.com\"\n}" },
    ],
    status: "running",
  },
  {
    defaultOpen: false,
    id: "tool-deep-research",
    input: "tool call UI patterns",
    intent: "Avvio una ricerca approfondita sui pattern di tool call.",
    name: "deepResearch",
    result: { output: "jobId: research-001", status: "success" },
    status: "success",
  },
  {
    defaultOpen: false,
    id: "tool-research-status",
    input: "research-001",
    intent: "Controllo lo stato della ricerca avviata.",
    name: "researchStatus",
    result: { output: "status: completed", status: "success" },
    status: "success",
  },
] satisfies FakeChatToolCall[]).map((tool, index) => ({
  ...tool,
  checkpointSeq: index + 2,
}));

// Deliberately UI-only fixtures. They are never written to ~/.solomon and let
// us iterate on the conversation surface before wiring the worker/API.
const initialFakeChatFixtures: FakeChat[] = [
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
      { id: "brief-user-10", role: "user", content: "Vorrei includere anche una verifica su immagini, file allegati e riferimenti a percorsi del progetto, perché sono elementi importanti per il lavoro quotidiano.", images: [{ name: "product-brief-reference.svg", url: "/vscode-icons/file_type_image.svg" }] },
      { id: "brief-assistant-10", role: "assistant", content: "## Allegati e riferimenti\n\nPer immagini, file e percorsi del progetto distinguerei sempre tra **contenuto disponibile**, **contenuto in caricamento** e **contenuto non accessibile**. La risposta non dovrebbe trattare questi tre casi nello stesso modo.\n\n| Caso | Cosa mostrare | Azione possibile |\n| --- | --- | --- |\n| File pronto | nome, tipo e dimensione | aprire o rimuovere |\n| Upload in corso | progresso e stato | annullare |\n| File mancante | errore comprensibile | ritentare o sostituire |\n| Percorso non valido | percorso ricevuto | correggere riferimento |\n\nAggiungerei test per:\n\n- immagini incollate e immagini troppo grandi;\n- file mancanti o duplicati;\n- percorsi relativi e assoluti;\n- riferimenti allo stesso documento in più messaggi;\n- allegati rimossi mentre la risposta è ancora in streaming.\n\nPer i riferimenti al codice, un link come [`gui/src/chat-test/fakeChats.ts`](../chat-test/fakeChats.ts) dovrebbe restare leggibile anche quando il messaggio viene riaperto. L’errore deve spiegare **che cosa manca** e **come risolverlo**, non limitarsi a dire “fallito”." },
      { id: "brief-user-11", role: "user", content: "Puoi chiudere con un riepilogo operativo che il team possa usare durante la riunione di go-live?" },
      { id: "brief-assistant-11", role: "assistant", content: "## Riepilogo operativo\n\n### Prima della finestra\n\n- [ ] confermare responsabili e orario;\n- [ ] validare test, metriche e baseline;\n- [ ] provare il rollback;\n- [ ] controllare streaming, allegati e persistenza;\n- [ ] preparare monitoraggio e comunicazioni.\n\n### Durante il rollout\n\nProcederei per percentuali crescenti:\n\n1. 10% degli utenti;\n2. controllo degli errori e della latenza;\n3. 25% se i segnali sono stabili;\n4. nuovo controllo prima del 50%;\n5. estensione completa solo dopo l’ultima decisione.\n\n> Go solo se il percorso principale funziona, il rollback è eseguibile e ogni segnale rosso ha un proprietario.\n\n**Decisione finale:** procedere a piccoli passi, registrando dopo ogni fase cosa è stato osservato, quale scelta è stata fatta e perché." },
      { id: "brief-user-12", role: "user", content: "Per la fase pilota, quali informazioni raccoglieresti direttamente dagli utenti oltre alle metriche tecniche?" },
      { id: "brief-assistant-12", role: "assistant", content: "Raccoglierei il punto in cui l’utente ha completato o abbandonato il flusso, la chiarezza percepita dei passaggi e un commento libero sui momenti di esitazione. Una domanda breve dopo il primo utilizzo sarebbe più utile di un questionario lungo: **che cosa ti ha aiutato e che cosa ti ha rallentato?**" },
      { id: "brief-user-13", role: "user", content: "Quali decisioni dovremmo prendere prima di aprire il pilota, così da non improvvisare durante la raccolta dei feedback?" },
      { id: "brief-assistant-13", role: "assistant", content: "Definirei in anticipo il pubblico, la durata, il canale per i feedback e il proprietario di ogni segnale. Fisserei anche la soglia che fa scattare una correzione e il momento in cui il team si riunisce per decidere se continuare." },
      { id: "brief-user-14", role: "user", content: "Aggiungi una nota su come distinguere un problema del prodotto da un problema di onboarding o di comunicazione." },
      { id: "brief-assistant-14", role: "assistant", content: "Confronterei il comportamento tra utenti che hanno ricevuto la stessa introduzione e utenti che hanno seguito un percorso diverso. Se l’errore scompare con istruzioni più chiare, è un problema di onboarding; se resta anche dopo la spiegazione, va trattato come problema del prodotto." },
      { id: "brief-user-15", role: "user", content: "Ora vorrei una traccia breve per la riunione di chiusura del pilota, con decisioni, evidenze e azioni successive." },
      { id: "brief-assistant-15", role: "assistant", content: "La riunione dovrebbe seguire questo ordine: **obiettivo**, risultati rispetto alla baseline, feedback ricorrenti, incidenti e mitigazioni, decisione sul rollout e azioni con responsabile e scadenza. Chiuderei con un verbale di una pagina, così la decisione resta verificabile anche per chi non era presente." },
      {
        id: "brief-compaction",
        kind: "compaction",
        role: "assistant",
        content: "",
        summary: "La conversazione ha definito un piano di rilascio graduale per la nuova esperienza chat. L’obiettivo è ridurre il tempo necessario per arrivare al primo messaggio utile, mantenendo chiari il contesto, lo stato della risposta e le azioni disponibili quando qualcosa si interrompe.\n\n**Responsabilità e flusso**\n\nIl percorso principale comprende apertura della chat, scelta del contesto, primo messaggio, risposta e nuova richiesta. Prodotto, engineering e supporto devono avere un responsabile esplicito; una persona dedicata raccoglie le evidenze e coordina la decisione di go/no-go.\n\n**Decisioni prese**\n\n- procedere per percentuali crescenti, dal 10% al 25%, 50% e 100%;\n- confrontare ogni fase con la baseline precedente;\n- conservare un riepilogo leggibile quando la conversazione supera il contesto disponibile;\n- distinguere contenuto pronto, contenuto in caricamento e contenuto non accessibile;\n- rendere verificabili interruzioni, retry, allegati e riapertura della conversazione.\n\n**Criteri di rilascio**\n\nIl pilota deve raccogliere sia segnali tecnici sia feedback sull’esperienza: tempo al primo token, success rate, errori, timeout, costi, qualità percepita e punti di abbandono. Un segnale giallo ricorrente rallenta il rollout; un segnale rosso sul percorso principale lo interrompe.\n\nPrima della finestra devono essere completati smoke test, verifica responsive, prova di rollback, controllo della persistenza e preparazione del monitoraggio. Il rollback deve avere una versione stabile già verificata, un proprietario nominato e una procedura abbastanza breve da poter essere eseguita durante un incidente reale.\n\nAl termine del pilota il team confronterà risultati, feedback ricorrenti, incidenti e mitigazioni. La decisione finale dovrà indicare se correggere, rallentare o proseguire il rollout, con un’azione assegnata e una scadenza per ogni punto ancora aperto.",
        retainedMessages: [
          { role: "user", content: "Per la fase pilota, quali informazioni raccoglieresti direttamente dagli utenti oltre alle metriche tecniche?" },
          { role: "assistant", content: "Raccoglierei il punto in cui l’utente ha completato o abbandonato il flusso, la chiarezza percepita dei passaggi e un commento libero sui momenti di esitazione." },
          { role: "user", content: "Quali decisioni dovremmo prendere prima di aprire il pilota, così da non improvvisare durante la raccolta dei feedback?" },
          { role: "assistant", content: "Definirei in anticipo il pubblico, la durata, il canale per i feedback e il proprietario di ogni segnale." },
          { role: "user", content: "Aggiungi una nota su come distinguere un problema del prodotto da un problema di onboarding o di comunicazione." },
          { role: "assistant", content: "Confronterei il comportamento tra utenti che hanno ricevuto la stessa introduzione e utenti che hanno seguito un percorso diverso." },
          { role: "user", content: "Ora vorrei una traccia breve per la riunione di chiusura del pilota, con decisioni, evidenze e azioni successive." },
          { role: "assistant", content: "La riunione dovrebbe seguire questo ordine: obiettivo, risultati rispetto alla baseline, feedback, incidenti, decisione sul rollout e azioni con responsabile e scadenza." },
        ],
      },
    ],
  },
  {
    id: "test-tool-result",
    modeSwitchTarget: "agent",
    title: "Tool disponibili",
    messages: [
      { checkpointSeq: 0, id: "tool-user", role: "user", content: "Quali elementi deve mostrare un turno dell’agente?" },
      {
        checkpointSeq: 1,
        id: "tool-assistant",
        role: "assistant",
        reasoning: "Prima chiarirei il risultato atteso del turno, poi sceglierei il passaggio minimo necessario per verificarlo. Il ragionamento resta separato dall’output così il percorso è leggibile senza confondersi con la risposta finale.",
        thoughtFor: 2.18,
        workedFor: 6.42,
        toolCalls: allAvailableFakeToolCalls,
        content: "## Un turno osservabile\n\nUn turno dell’agente dovrebbe rendere comprensibile sia **che cosa sta facendo** sia **che cosa ha ottenuto**. Mostrerei questi elementi nell’ordine in cui accadono:\n\n1. messaggio dell’utente;\n2. reasoning o piano sintetico;\n3. attività dei tool;\n4. risultato del tool;\n5. risposta finale;\n6. errori e metriche del turno.\n\n| Elemento | Serve a | Esempio |\n| --- | --- | --- |\n| Reasoning | spiegare il prossimo passo | “Cerco il file di configurazione” |\n| Tool call | mostrare l’azione | `read_file(path)` |\n| Risultato | rendere verificabile l’azione | file trovato, 84 righe |\n| Stato | indicare come è finito il turno | completato o interrotto |\n\nUn risultato tecnico potrebbe essere rappresentato così:\n\n```json\n{\n  \"status\": \"completed\",\n  \"tool\": \"read_file\",\n  \"items\": 3\n}\n```\n\n> La trasparenza non significa mostrare ogni dettaglio interno: significa lasciare abbastanza tracce perché l’utente possa capire e verificare il lavoro."
      },
      { checkpointSeq: 28, id: "tool-user-interrupted", role: "user", content: "E se interrompessi la risposta a metà?" },
      { checkpointSeq: 28, id: "tool-assistant-interrupted", role: "assistant", status: "interrupted", workedFor: 1.83, content: "Mostrerei il testo già generato e chiuderei il turno con uno stato esplicito, così è chiaro che la risposta non è completa." },
    ],
  },
];

export const initialFakeChats: FakeChat[] = initialFakeChatFixtures.map(addFixtureWorkedFor);

function addFixtureWorkedFor(chat: FakeChat): FakeChat {
  return {
    ...chat,
    messages: chat.messages.map((message) => {
      if (message.role !== "assistant" || message.kind === "compaction") return message;
      return {
        ...message,
        reasoning: message.reasoning ?? estimateFakeReasoning(message.content),
        stats: message.stats ?? estimateFakeStats(message.content),
        thoughtFor: message.thoughtFor ?? estimateFakeThoughtFor(message.content),
        workedFor: message.workedFor ?? estimateFakeWorkedFor(message.content),
      };
    }),
  };
}

function estimateFakeReasoning(content: string) {
  const normalized = content.toLowerCase();
  if (normalized.includes("risch") || normalized.includes("mitig")) return "Individuo i rischi principali e li collego a mitigazioni verificabili.";
  if (normalized.includes("rollback") || normalized.includes("riprist")) return "Ordino i passaggi operativi e verifico che il percorso possa essere ripreso senza improvvisare.";
  if (normalized.includes("metric") || normalized.includes("segnal")) return "Separo gli obiettivi dai segnali osservabili, così la risposta resta misurabile.";
  if (normalized.includes("immagin") || normalized.includes("allegat")) return "Controllo i riferimenti disponibili e distinguo il contenuto pronto da quello che richiede un’azione.";
  if (normalized.includes("interromp") || normalized.includes("timeout")) return "Considero il caso di interruzione e rendo esplicito quale stato può essere recuperato.";
  return "Raccolgo i vincoli della richiesta e organizzo una risposta verificabile prima di mostrarne l’output.";
}

function estimateFakeThoughtFor(content: string) {
  const seconds = 0.42 + content.trim().length / 900;
  return Number(Math.min(4.8, seconds).toFixed(2));
}

function estimateFakeWorkedFor(content: string) {
  const seconds = 0.84 + content.trim().length / 240;
  return Number(Math.min(12.8, seconds).toFixed(2));
}

export function estimateFakeStats(content: string): FakeChatStats {
  const responseTokens = Math.max(1, Math.ceil(content.trim().length / 4));
  const reasoningTokens = Math.max(24, Math.round(responseTokens * 0.36));
  const contextTokens = 640 + Math.min(3200, responseTokens * 2);
  const userTokens = Math.max(8, Math.ceil(content.trim().length / 18));
  const totalTokens = contextTokens + userTokens + reasoningTokens + responseTokens;
  const workedFor = estimateFakeWorkedFor(content);
  const promptDuration = Math.max(0.7, 0.48 + contextTokens / 1600);

  return {
    contextTokens,
    outputTokensPerSecond: Number((responseTokens / Math.max(0.3, workedFor)).toFixed(1)),
    promptTokensPerSecond: Number(((contextTokens + userTokens) / promptDuration).toFixed(1)),
    reasoningTokens,
    responseTokens,
    totalTokens,
    ttftSeconds: Number((0.42 + responseTokens / 1600).toFixed(2)),
    userTokens,
  };
}

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
  const createdAt = Date.now();
  const quotedText = text.trim().split(/\r?\n/).map((line) => `> ${line || " "}`).join("\n");
  const content = `## Risposta di test\n\nHo ricevuto la richiesta e l’ho aggiunta al turno simulato. Per rendere visibile il comportamento del transcript, qui sotto mostro sia l’interpretazione sia l’output prodotto.\n\n### Richiesta originale\n\n${quotedText}\n\n### Interpretazione\n\n- il messaggio è stato ricevuto correttamente;\n- il contenuto resta disponibile nel contesto del turno;\n- la risposta seguente è solo un esempio, non un risultato del runtime reale.\n\n### Output simulato\n\n~~~text\nparse request\ncreate assistant turn\nrender markdown\n~~~\n\n**Stato:** risposta simulata\n\n> Il collegamento al runtime Solomon arriverà nel prossimo passaggio.\n\nSe vuoi, puoi inviare un altro messaggio usando \`**grassetto**\`, una lista o un blocco di codice per verificare il rendering in tempo reale.`;

  return {
    createdAt,
    id: `assistant-${createdAt}`,
    role: "assistant",
    reasoning: "Raccolgo la richiesta, verifico il formato atteso e preparo una risposta di test separando il percorso dall’output.",
    stats: estimateFakeStats(content),
    thoughtFor: 0.84,
    workedFor: estimateFakeWorkedFor(text),
    content,
  };
}
