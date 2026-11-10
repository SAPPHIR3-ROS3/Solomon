import type { LanguageSettings } from "./languages";

export type Position = { line: number; character: number };
export type Range = { start: Position; end: Position };
export type Diagnostic = { range: Range; message: string; severity?: number };
export type Capabilities = {
  positionEncoding?: string;
  textDocumentSync?: number | { change?: number; save?: boolean | { includeText?: boolean } };
  completionProvider?: { triggerCharacters?: string[] }; hoverProvider?: boolean | object;
};

type Message = { id?: string | number; method?: string; params?: any; result?: any; error?: { message: string } };

export class LanguageConnection {
  private sequence = 0;
  private pending = new Map<number, { resolve: (value: any) => void; reject: (error: Error) => void; timer: ReturnType<typeof setTimeout> }>();
  private closed = false;
  private closing = false;
  onDiagnostics: (diagnostics: Diagnostic[], version?: number) => void = () => {};
  onClose: (reason: string) => void = () => {};

  constructor(private socket: WebSocket, private settings: LanguageSettings, private uri: string) {
    socket.addEventListener("message", event => {
      try { this.receive(JSON.parse(event.data)); } catch { this.close(); }
    });
    socket.addEventListener("close", () => this.disconnected("Language server disconnected"));
    socket.addEventListener("error", () => this.disconnected("Language server unavailable"));
  }

  async open(rootUri: string): Promise<Capabilities> {
    if (this.socket.readyState !== WebSocket.OPEN) await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => { this.socket.close(); reject(new Error("Language server connection timed out")); }, 10000);
      this.socket.addEventListener("open", () => { clearTimeout(timer); resolve(); }, { once: true });
      this.socket.addEventListener("error", () => { clearTimeout(timer); reject(new Error("Language server connection failed")); }, { once: true });
      this.socket.addEventListener("close", () => { clearTimeout(timer); reject(new Error("Language server connection closed")); }, { once: true });
    });
    const result = await this.request("initialize", {
      processId: null, clientInfo: { name: "Solomon" }, rootUri,
      workspaceFolders: [{ uri: rootUri, name: rootUri.split("/").at(-1) ?? "Workspace" }],
      initializationOptions: this.settings.initializationOptions ?? {},
      capabilities: {
        general: { positionEncodings: ["utf-16"] }, workspace: { configuration: true },
        textDocument: { synchronization: { didSave: true }, completion: { completionItem: { snippetSupport: false } },
          hover: { contentFormat: ["plaintext"] }, publishDiagnostics: { versionSupport: true } },
      },
    });
    if (this.closed) throw new Error("Language server disconnected during initialization");
    if (result.capabilities?.positionEncoding && result.capabilities.positionEncoding !== "utf-16") throw new Error("Language server requires an unsupported position encoding");
    this.notify("initialized", {});
    this.notify("workspace/didChangeConfiguration", { settings: this.settings.settings ?? {} });
    return result.capabilities ?? {};
  }

  request(method: string, params: unknown): Promise<any> {
    if (this.closed) return Promise.reject(new Error("Language server is closed"));
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); this.notify("$/cancelRequest", { id }); reject(new Error(`${method} timed out`)); }, 15000);
      this.pending.set(id, { resolve, reject, timer });
      try { this.socket.send(JSON.stringify({ jsonrpc: "2.0", id, method, params })); }
      catch (error) { clearTimeout(timer); this.pending.delete(id); reject(error); }
    });
  }

  notify(method: string, params: unknown) {
    if (this.socket.readyState === WebSocket.OPEN) this.socket.send(JSON.stringify({ jsonrpc: "2.0", method, params }));
  }

  private receive(message: Message) {
    if (message.method) {
      if (message.method === "textDocument/publishDiagnostics" && message.params?.uri === this.uri) this.onDiagnostics(message.params.diagnostics ?? [], message.params.version);
      if (message.id !== undefined) {
        let result: unknown;
        if (message.method === "workspace/configuration") result = (message.params?.items ?? []).map((item: { section?: string }) => {
          if (!item.section) return this.settings.settings ?? {};
          return item.section.split(".").reduce<any>((value, key) => value?.[key], this.settings.settings ?? {}) ?? null;
        });
        else if (message.method === "workspace/applyEdit") result = { applied: false, failureReason: "Server-initiated workspace edits are not supported" };
        else if (["client/registerCapability", "client/unregisterCapability", "window/workDoneProgress/create"].includes(message.method)) result = null;
        else { this.socket.send(JSON.stringify({ jsonrpc: "2.0", id: message.id, error: { code: -32601, message: "Unsupported client request" } })); return; }
        this.socket.send(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
      }
      return;
    }
    const pending = this.pending.get(Number(message.id));
    if (!pending) return;
    clearTimeout(pending.timer); this.pending.delete(Number(message.id));
    if (message.error) pending.reject(new Error(message.error.message)); else pending.resolve(message.result);
  }

  private disconnected(reason: string) {
    if (this.closed) return;
    this.closed = true;
    for (const task of this.pending.values()) { clearTimeout(task.timer); task.reject(new Error(reason)); }
    this.pending.clear(); this.onClose(reason);
  }

  close() {
    if (this.closed || this.closing || this.socket.readyState !== WebSocket.OPEN) { this.socket.close(); return; }
    this.closing = true;
    this.onDiagnostics = () => {};
    this.notify("textDocument/didClose", { textDocument: { uri: this.uri } });
    const timer = setTimeout(() => { this.socket.close(); this.disconnected("Language server closed"); }, 500);
    void this.request("shutdown", null).catch(() => {}).finally(() => {
      clearTimeout(timer); this.notify("exit", null); this.socket.close(); this.disconnected("Language server closed");
    });
  }
}
