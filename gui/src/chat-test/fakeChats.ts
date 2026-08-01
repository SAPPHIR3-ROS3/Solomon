export type FakeChatMessage = {
  id: string;
  role: "assistant" | "user";
  content: string;
};

export type FakeChat = {
  id: string;
  messages: FakeChatMessage[];
  title: string;
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
      { id: "brief-assistant", role: "assistant", content: "Certo. Partirei da obiettivo del rilascio, pubblico pilota, flusso principale dall’apertura della chat all’invio del primo messaggio, metriche osservabili e piano di comunicazione per il team." },
      { id: "brief-user-2", role: "user", content: "Aggiungi anche una parte dedicata ai rischi tecnici e operativi, con un’indicazione concreta di come ridurli prima di esporre la funzione a tutti gli utenti." },
      { id: "brief-assistant-2", role: "assistant", content: "Rischi principali: regressioni non coperte, dipendenze esterne indisponibili, tempi di risposta troppo lunghi, perdita dello stato della conversazione e assenza di un piano di rollback. Per ciascuno assocerei un test, un proprietario e una soglia di allarme." },
      { id: "brief-user-3", role: "user", content: "Come organizzeresti la fase di verifica in modo che sia ripetibile anche nei prossimi rilasci e non dipenda solo da controlli manuali fatti all’ultimo momento?" },
      { id: "brief-assistant-3", role: "assistant", content: "Userei una checklist versionata per ambiente, test automatici sulle API, smoke test manuale sul percorso principale, verifica responsive e un controllo finale dei log, delle metriche e degli errori più frequenti." },
      { id: "brief-user-4", role: "user", content: "Quali metriche mostreresti al team durante il rollout e quali segnali useresti per decidere se continuare, rallentare o interrompere la distribuzione?" },
      { id: "brief-assistant-4", role: "assistant", content: "Success rate, latenza p95, errori per endpoint, tempo al primo token, completamento del flusso principale e feedback qualitativo degli utenti pilota. Confronterei ogni valore con la baseline precedente e con una soglia di rollback concordata." },
      { id: "brief-user-5", role: "user", content: "Serve anche una strategia per il rollback? Vorrei che fosse chiara abbastanza da poter essere eseguita rapidamente anche durante un incidente reale." },
      { id: "brief-assistant-5", role: "assistant", content: "Sì. Definirei il criterio di annullamento, il responsabile della decisione, la versione stabile precedente, i comandi o passi necessari e un controllo post-rollback per verificare che il servizio sia tornato nello stato atteso." },
      { id: "brief-user-6", role: "user", content: "Chi dovrebbe approvare il rilascio e quali informazioni minime dovrebbe avere prima di dare il via libera definitivo?" },
      { id: "brief-assistant-6", role: "assistant", content: "Un referente prodotto e uno tecnico dovrebbero approvare insieme, dopo aver controllato checklist, metriche della fase pilota, rischi residui, piano di rollback, responsabilità operative e messaggio da condividere con gli utenti." },
      { id: "brief-user-7", role: "user", content: "Perfetto, chiudiamo con i prossimi passi ordinati per priorità, indicando cosa deve essere fatto prima della finestra di rilascio e cosa invece può essere completato subito dopo." },
      { id: "brief-assistant-7", role: "assistant", content: "Prima del rilascio: assegnare i responsabili, fissare la finestra, completare smoke test e verificare il rollback. Durante il rollout: monitorare errori e latenza. Dopo il lancio: raccogliere feedback, confrontare le metriche con la baseline e registrare le decisioni per il prossimo ciclo." },
      { id: "brief-user-8", role: "user", content: "Aggiungi un controllo specifico per la qualità della risposta e per i casi in cui l’utente interrompe il flusso prima della fine." },
      { id: "brief-assistant-8", role: "assistant", content: "Inserirei test per risposte incomplete, interruzioni durante lo streaming, timeout, retry e ripresa della conversazione. Registrerei anche se l’interruzione è stata volontaria o causata da un errore tecnico." },
      { id: "brief-user-9", role: "user", content: "Come gestiresti invece i messaggi molto lunghi e le conversazioni che non entrano più nel contesto disponibile?" },
      { id: "brief-assistant-9", role: "assistant", content: "Mostrerei uno stato di elaborazione esplicito, applicherei una strategia di compaction controllata e conserverei un riepilogo leggibile. La UI dovrebbe mantenere il punto di lettura e indicare quando parte del contesto è stata riassunta." },
      { id: "brief-user-10", role: "user", content: "Vorrei includere anche una verifica su immagini, file allegati e riferimenti a percorsi del progetto, perché sono elementi importanti per il lavoro quotidiano." },
      { id: "brief-assistant-10", role: "assistant", content: "Aggiungerei casi con immagini incollate, file mancanti, percorsi non validi e riferimenti multipli allo stesso documento. Ogni allegato dovrebbe avere preview, stato di caricamento, errore comprensibile e comportamento prevedibile nel transcript." },
      { id: "brief-user-11", role: "user", content: "Puoi chiudere con un riepilogo operativo che il team possa usare durante la riunione di go-live?" },
      { id: "brief-assistant-11", role: "assistant", content: "Riepilogo: confermare responsabili e finestra, validare test e metriche, provare il rollback, controllare streaming e allegati, preparare monitoraggio e comunicazioni, quindi procedere per percentuali crescenti con una decisione esplicita a ogni fase." },
    ],
  },
  {
    id: "test-tool-result",
    title: "Tool e risultati",
    messages: [
      { id: "tool-user", role: "user", content: "Quali elementi deve mostrare un turno dell’agente?" },
      { id: "tool-assistant", role: "assistant", content: "Messaggio, reasoning, attività dei tool, risultato, errori e metriche del turno." },
    ],
  },
  {
    id: "test-empty",
    title: "Chat vuota",
    messages: [],
  },
];

export function fakeAssistantReply(text: string): FakeChatMessage {
  return {
    id: `assistant-${Date.now()}`,
    role: "assistant",
    content: `Risposta di test ricevuta: “${text.trim()}”. Il collegamento al runtime Solomon arriverà nel prossimo passaggio.`,
  };
}
