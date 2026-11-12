import { EditorState, RangeSetBuilder } from "@codemirror/state";
import { Decoration, EditorView, ViewPlugin, type ViewUpdate } from "@codemirror/view";
import policy from "../../../tui/syntax/display.json";

export function leadingDisplayIndent(line: string): number {
  let columns = 0;
  for (const character of line) {
    if (character === " ") columns++;
    else if (character === "\t") columns += policy.indent_size - columns % policy.indent_size;
    else break;
  }
  return columns;
}

function indentationDecorations(view: EditorView) {
  const builder = new RangeSetBuilder<Decoration>();
  for (let index = 1; index <= view.state.doc.lines; index++) {
    const line = view.state.doc.line(index);
    const indent = leadingDisplayIndent(line.text);
    if (!indent) continue;
    builder.add(line.from, line.from, Decoration.line({ attributes: {
      style: `padding-left:calc(8px + ${indent}ch);text-indent:-${indent}ch`,
    } }));
  }
  return builder.finish();
}

export function syntaxDisplayPolicy() {
  return [
    EditorState.tabSize.of(policy.indent_size),
    EditorView.lineWrapping,
    EditorView.theme({ ".cm-content": {
      fontVariantLigatures: policy.ligatures ? "normal" : "none",
      fontFeatureSettings: policy.ligatures ? "normal" : '"liga" 0, "calt" 0',
    } }),
    ViewPlugin.fromClass(class {
      decorations;
      constructor(view: EditorView) { this.decorations = indentationDecorations(view); }
      update(update: ViewUpdate) {
        if (update.docChanged) this.decorations = indentationDecorations(update.view);
      }
    }, { decorations: value => value.decorations }),
  ];
}
