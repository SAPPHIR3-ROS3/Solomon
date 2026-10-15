import { RangeSetBuilder, StateEffect } from "@codemirror/state";
import { Decoration, EditorView, ViewPlugin, type DecorationSet, type ViewUpdate } from "@codemirror/view";
import { cursorForeground, cursorTokenizer, cursorTokenStyle } from "./cursorSyntax";

const highlightedTokens = StateEffect.define<DecorationSet>();

export function cursorEditorHighlighting(pathOrLanguage: string) {
  return [EditorView.theme({ ".cm-content": { color: cursorForeground } }), ViewPlugin.fromClass(class {
    decorations: DecorationSet = Decoration.none;
    tokenizer: Awaited<ReturnType<typeof cursorTokenizer>> | undefined;
    destroyed = false;
    frame: number | undefined;

    constructor(private view: EditorView) {
      void cursorTokenizer(pathOrLanguage, view.state.doc.line(1).text).then(tokenizer => {
        if (this.destroyed) return;
        this.tokenizer = tokenizer;
        this.schedule();
      }).catch(error => console.error("Unable to initialize syntax highlighting", error));
    }

    update(update: ViewUpdate) {
      if (update.docChanged) {
        this.decorations = this.decorations.map(update.changes);
        this.schedule();
      }
      for (const transaction of update.transactions) {
        for (const effect of transaction.effects) {
          if (effect.is(highlightedTokens)) this.decorations = effect.value;
        }
      }
    }

    schedule() {
      if (!this.tokenizer?.highlighter || this.frame !== undefined) return;
      this.frame = requestAnimationFrame(() => {
        this.frame = undefined;
        if (this.destroyed || !this.tokenizer?.highlighter) return;
        const builder = new RangeSetBuilder<Decoration>();
        const { highlighter, language } = this.tokenizer;
        const tokens = highlighter.codeToTokens(this.view.state.doc.toString(), { lang: language, theme: "cursor-dark" }).tokens;
        for (let index = 0; index < tokens.length && index < this.view.state.doc.lines; index++) {
          let from = this.view.state.doc.line(index + 1).from;
          for (const token of tokens[index]) {
            const to = from + token.content.length;
            if (to > from) builder.add(from, to, Decoration.mark({ class: "cm-cursor-token", attributes: { style: cursorTokenStyle(token) } }));
            from = to;
          }
        }
        // An effect redraws the tokens without changing the document or selection.
        this.view.dispatch({ effects: highlightedTokens.of(builder.finish()) });
      });
    }

    destroy() {
      this.destroyed = true;
      if (this.frame !== undefined) cancelAnimationFrame(this.frame);
    }
  }, { decorations: plugin => plugin.decorations })];
}
