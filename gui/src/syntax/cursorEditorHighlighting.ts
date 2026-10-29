import { RangeSetBuilder, StateEffect } from "@codemirror/state";
import { Decoration, EditorView, ViewPlugin, type DecorationSet, type ViewUpdate } from "@codemirror/view";
import { cursorForeground, cursorTokenizer, cursorTokenStyle } from "./cursorSyntax";
import { rainbowBrackets } from "./rainbowBrackets";

const highlightedTokens = StateEffect.define<DecorationSet>();

export function cursorEditorHighlighting(pathOrLanguage: string) {
  return [EditorView.theme({ ".cm-content": { color: cursorForeground } }), ViewPlugin.fromClass(class {
    decorations: DecorationSet = Decoration.none;
    tokenizer: Awaited<ReturnType<typeof cursorTokenizer>> | undefined;
    destroyed = false;
    timer: ReturnType<typeof setTimeout> | undefined;

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
      if (!this.tokenizer) return;
      if (this.timer !== undefined) clearTimeout(this.timer);
      // Keep full-document tokenization off the key-repeat/render path.
      // Existing decorations are mapped through edits until input settles.
      this.timer = setTimeout(() => {
        this.timer = undefined;
        if (this.destroyed || !this.tokenizer) return;
        const builder = new RangeSetBuilder<Decoration>();
        const { highlighter, language } = this.tokenizer;
        const source = this.view.state.doc.toString();
        const tokens = rainbowBrackets(highlighter
          ? highlighter.codeToTokens(source, { lang: language, theme: "cursor-dark", includeExplanation: "tokenType" }).tokens
          : source.split("\n").map(content => [{ content, offset: 0 }]));
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
      }, 150);
    }

    destroy() {
      this.destroyed = true;
      if (this.timer !== undefined) clearTimeout(this.timer);
    }
  }, { decorations: plugin => plugin.decorations })];
}
