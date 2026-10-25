import { useEffect, useLayoutEffect, useRef } from "react";
import { closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";
import { defaultKeymap, insertNewlineAndIndent } from "@codemirror/commands";
import { indentUnit } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { cursorEditorHighlighting } from "../syntax/cursorEditorHighlighting";
import { cursorForeground } from "../syntax/cursorSyntax";
import type { OpenFile, EditorViewState } from "./editorSession";

export function CodeEditor({ file, onChange, onCursor, onSave, viewState, onViewState }: {
  file: OpenFile;
  onChange: (content: string) => void;
  onCursor: (cursor: { line: number; column: number }) => void;
  onSave: (path: string, content: string) => void;
  viewState?: EditorViewState;
  onViewState: (state: EditorViewState) => void;
}) {
  const viewRef = useRef<EditorView | null>(null);
  const host = useRef<HTMLDivElement>(null);
  const callbacks = useRef({ onChange, onCursor, onSave, onViewState });
  callbacks.current = { onChange, onCursor, onSave, onViewState };
  useLayoutEffect(() => {
    if (!host.current) return;
    let restoring = Boolean(viewState);
    let frame: number | undefined;
    const saveView = (view: EditorView) => {
      if (restoring) return;
      const { anchor, head } = view.state.selection.main;
      const snapshot = view.scrollSnapshot().value;
      callbacks.current.onViewState({ top: view.scrollDOM.scrollTop, left: view.scrollDOM.scrollLeft, anchor, head, scrollAnchor: snapshot.range.head, scrollOffset: snapshot.yMargin });
    };
    const view = new EditorView({ parent: host.current,
      scrollTo: viewState?.scrollAnchor !== undefined ? EditorView.scrollIntoView(Math.min(file.content.length, viewState.scrollAnchor), { y: "start", yMargin: viewState.scrollOffset ?? 0, x: "start", xMargin: viewState.left }) : undefined,
      state: EditorState.create({
      doc: file.content,
      selection: viewState ? { anchor: Math.min(file.content.length, viewState.anchor), head: Math.min(file.content.length, viewState.head) } : undefined,
      extensions: [
        lineNumbers(), ...cursorEditorHighlighting(file.path), EditorView.lineWrapping,
        indentUnit.compute(["doc"], state => /^\t/m.test(state.doc.toString()) ? "\t" : "  "), closeBrackets(),
        EditorView.scrollHandler.of((editor, range, options) => {
          if (!restoring || viewState?.scrollAnchor === undefined || range.head !== Math.min(editor.state.doc.length, viewState.scrollAnchor) || options.y !== "start") return false;
          // Restore the saved line block and pixel offset, rather than the
          // text baseline used by a normal scrollIntoView request.
          editor.scrollDOM.scrollTop = editor.lineBlockAt(range.head).top - (viewState.scrollOffset ?? 0);
          editor.scrollDOM.scrollLeft = viewState.left;
          return true;
        }),
        EditorView.domEventHandlers({ scroll: (_event, editor) => { saveView(editor); } }),
        EditorView.updateListener.of(update => {
          if (update.docChanged) callbacks.current.onChange(update.state.doc.toString());
          if (update.selectionSet || update.docChanged) {
            const pos = update.state.selection.main.head;
            const line = update.state.doc.lineAt(pos);
            callbacks.current.onCursor({ line: line.number, column: pos - line.from + 1 });
            saveView(update.view);
          }
        }),
        keymap.of([
          ...closeBracketsKeymap,
          { key: "Enter", preventDefault: true, run: insertNewlineAndIndent, shift: insertNewlineAndIndent },
          { key: "Mod-s", preventDefault: true, run: editor => { callbacks.current.onSave(file.path, editor.state.doc.toString()); return true; } },
          ...defaultKeymap,
        ]),
        EditorView.theme({
          "&": { height: "100%", background: "transparent", color: cursorForeground },
          ".cm-scroller": { overflow: "auto", fontFamily: 'var(--font-mono)' },
          ".cm-content": { padding: "24px 0 130px", caretColor: "#82aaff" },
          ".cm-line": { padding: "0 28px 0 8px" },
          ".cm-gutters": { minWidth: "64px", background: "transparent", border: "none", color: "#52616a" },
          ".cm-gutterElement": { transform: null },
          "&.cm-focused": { outline: "none" },
          ".cm-selectionBackground, ::selection": { background: "#82aaff55 !important" },
        }, { dark: true }),
      ],
    }) });
    viewRef.current = view;
    const position = view.state.selection.main.head;
    const line = view.state.doc.lineAt(position);
    callbacks.current.onCursor({ line: line.number, column: position - line.from + 1 });
    if (viewState) {
      frame = requestAnimationFrame(() => {
        // Let CodeMirror measure its document before restoring the viewport.
        frame = requestAnimationFrame(() => {
          if (viewState.scrollAnchor === undefined) {
            view.scrollDOM.scrollTop = viewState.top;
            view.scrollDOM.scrollLeft = viewState.left;
          }
          restoring = false;
        });
      });
    }
    return () => {
      if (frame !== undefined) cancelAnimationFrame(frame);
      saveView(view);
      viewRef.current = null;
      view.destroy();
    };
  }, [file.id, file.path]);
  useEffect(() => {
    const view = viewRef.current;
    if (view && view.state.doc.toString() !== file.content) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: file.content } });
  }, [file.content]);
  return <div className="editor-code" ref={host} />;
}

