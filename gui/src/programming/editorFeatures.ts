import { autocompletion } from "@codemirror/autocomplete";
import { StateEffect } from "@codemirror/state";
import { Decoration, EditorView, hoverTooltip, ViewPlugin, type DecorationSet } from "@codemirror/view";
import type { Diagnostic, LanguageConnection, Position, Range } from "./connection";

export function positionAt(view: EditorView, offset: number): Position {
  const line = view.state.doc.lineAt(offset);
  return { line: line.number - 1, character: offset - line.from };
}
export function offsetAt(view: EditorView, position: Position): number {
  const line = view.state.doc.line(Math.max(1, Math.min(view.state.doc.lines, position.line + 1)));
  return Math.max(line.from, Math.min(line.to, line.from + position.character));
}

export const languageDiagnostics = StateEffect.define<Diagnostic[]>();
const diagnosticPlugin = ViewPlugin.fromClass(class {
  decorations: DecorationSet = Decoration.none;
  update(update: import("@codemirror/view").ViewUpdate) {
    if (update.docChanged) this.decorations = Decoration.none;
    for (const transaction of update.transactions) for (const effect of transaction.effects) {
      if (!effect.is(languageDiagnostics)) continue;
      const ranges = effect.value.slice(0, 2000).map(diagnostic => {
        const from = offsetAt(update.view, diagnostic.range.start), to = Math.max(from, offsetAt(update.view, diagnostic.range.end));
        const attributes = { title: diagnostic.message };
        return to > from ? Decoration.mark({ class: diagnostic.severity === 1 ? "language-error" : "language-warning", attributes }).range(from, to)
          : Decoration.line({ attributes: { ...attributes, class: "language-diagnostic-line" } }).range(update.state.doc.lineAt(from).from);
      });
      this.decorations = Decoration.set(ranges, true);
    }
  }
}, { decorations: plugin => plugin.decorations });

export function completionExtension(connection: LanguageConnection, uri: string, flush: () => void, triggers: string[]) {
  return autocompletion({ override: [async context => {
    const word = context.matchBefore(/[\w$]+/);
    if (!context.explicit && !word && !triggers.includes(context.state.sliceDoc(context.pos - 1, context.pos))) return null;
    const view = context.view;
    if (!view) return null;
    flush();
    const result = await connection.request("textDocument/completion", { textDocument: { uri }, position: positionAt(view, context.pos), context: { triggerKind: context.explicit ? 1 : 3 } }).catch(() => null);
    if (context.aborted || !result) return null;
    const items: Array<{ label: string; detail?: string; insertText?: string; textEdit?: { range?: Range; newText: string }; additionalTextEdits?: { range: Range; newText: string }[] }> = Array.isArray(result) ? result : result.items ?? [];
    return { from: word?.from ?? context.pos, options: items.slice(0, 300).map(item => ({
      label: item.label, detail: item.detail,
      apply: (editor: EditorView, _completion: unknown, from: number, to: number) => {
        const edit = item.textEdit;
        const main = { from: edit?.range ? offsetAt(editor, edit.range.start) : from, to: edit?.range ? offsetAt(editor, edit.range.end) : to, insert: edit?.newText ?? item.insertText ?? item.label };
        const changes = editor.state.changes([main, ...(item.additionalTextEdits ?? []).map(edit => ({ from: offsetAt(editor, edit.range.start), to: offsetAt(editor, edit.range.end), insert: edit.newText }))]);
        editor.dispatch({ changes, selection: { anchor: changes.mapPos(main.from, -1) + main.insert.length }, userEvent: "input.complete" });
      },
    })) };
  }] });
}

export function hoverExtension(connection: LanguageConnection, uri: string, flush: () => void) {
  return hoverTooltip(async (view, pos) => {
    flush();
    const result = await connection.request("textDocument/hover", { textDocument: { uri }, position: positionAt(view, pos) }).catch(() => null);
    if (!result?.contents) return null;
    const contents = Array.isArray(result.contents) ? result.contents : [result.contents];
    const text = contents.map((item: string | { value: string }) => typeof item === "string" ? item : item.value).join("\n\n");
    return { pos, create: () => { const dom = document.createElement("pre"); dom.className = "language-hover"; dom.textContent = text; return { dom }; } };
  });
}

export const languageFeatureTheme = [diagnosticPlugin, EditorView.theme({
  ".language-error": { textDecoration: "underline wavy #ef7183", textUnderlineOffset: "3px" },
  ".language-warning": { textDecoration: "underline wavy #e6b256", textUnderlineOffset: "3px" },
  ".language-diagnostic-line": { boxShadow: "inset 2px 0 #ef7183" },
  ".language-hover": { margin: "0", padding: "8px", maxWidth: "min(600px, 80vw)", whiteSpace: "pre-wrap", fontFamily: "var(--font-mono)" },
})];
