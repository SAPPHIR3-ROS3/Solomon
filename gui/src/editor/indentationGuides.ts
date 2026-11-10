import { countColumn, EditorSelection, EditorState, findColumn, RangeSetBuilder, StateField, type Text } from "@codemirror/state";
import { Decoration, EditorView } from "@codemirror/view";

function indentationPrefix(text: string, marker: string) {
  const whitespace = text.match(/^[\t ]*/)?.[0] ?? "";
  if (!marker || !text.startsWith(marker, whitespace.length)) return whitespace;
  const start = whitespace.length + marker.length;
  return text.slice(0, start) + (text.slice(start).match(/^[\t ]*/)?.[0] ?? "");
}

export function guideColumns(doc: Text, tabSize: number, marker = ""): number[][] {
  const indents = Array.from({ length: doc.lines }, (_, index) => {
    const text = doc.line(index + 1).text;
    const prefix = indentationPrefix(text, marker);
    return text.slice(prefix.length).trim() ? countColumn(prefix, tabSize) : undefined;
  });
  let next: number | undefined;
  const effective = indents.slice();
  for (let index = indents.length - 1; index >= 0; index--) {
    if (indents[index] !== undefined) next = indents[index];
    else effective[index] = next;
  }
  const stack: number[] = [];
  let previous = 0;
  return effective.map((value, index) => {
    const indent = value ?? previous;
    if (index && indent > previous) stack.push(previous);
    while (stack.length && stack[stack.length - 1] >= indent) stack.pop();
    previous = indent;
    return stack.slice();
  });
}

function buildGuides(state: EditorState, marker: string) {
  const columns = guideColumns(state.doc, state.tabSize, marker);
  const builder = new RangeSetBuilder<Decoration>();
  columns.forEach((guides, index) => {
    if (!guides.length) return;
    builder.add(state.doc.line(index + 1).from, state.doc.line(index + 1).from, Decoration.line({ attributes: {
      "data-indent-guides": guides.join(","),
      style: `background-image:${guides.map(() => "linear-gradient(var(--indent-guide-color),var(--indent-guide-color))").join(",")};background-position:${guides.map(column => `calc(8px + ${column}ch) 0`).join(",")};background-size:1px 100%;background-repeat:no-repeat`,
    } }));
  });
  return { columns, decorations: builder.finish() };
}

function clickGuide(event: MouseEvent, view: EditorView, columnsByLine: number[][], marker: string) {
  if (event.button !== 0 || event.altKey || event.shiftKey || view.state.readOnly) return false;
  const element = event.target instanceof Element ? event.target.closest(".cm-line") : null;
  if (!element || !view.contentDOM.contains(element)) return false;
  const line = view.state.doc.lineAt(view.posAtDOM(element));
  const columns = columnsByLine[line.number - 1];
  if (!columns.length) return false;
  const start = view.coordsAtPos(line.from);
  if (!start || event.clientY < start.top || event.clientY > start.bottom) return false;
  const clicked = (event.clientX - start.left) / view.defaultCharacterWidth;
  const column = columns.find(column => Math.abs(clicked - column) <= Math.min(0.4, 4 / view.defaultCharacterWidth));
  if (column === undefined) return false;
  const prefix = indentationPrefix(line.text, marker);
  const width = countColumn(prefix, view.state.tabSize);
  const offset = findColumn(prefix, column, view.state.tabSize);
  const before = countColumn(prefix, view.state.tabSize, offset);
  const insideTab = before > column;
  const tabStart = insideTab ? countColumn(prefix, view.state.tabSize, offset - 1) : 0;
  const insert = " ".repeat(insideTab ? before - tabStart : Math.max(0, column - width));
  const from = insideTab ? line.from + offset - 1 : line.from + prefix.length;
  const changes = { from, to: insideTab ? from + 1 : from, insert };
  const changeSet = view.state.changes(changes);
  const head = insideTab ? from + column - tabStart : width < column ? from + insert.length : line.from + offset;
  const additional = /Mac|iPhone|iPad|iPod/.test(view.dom.ownerDocument.defaultView?.navigator.platform ?? "") ? event.metaKey : event.ctrlKey;
  const selection = additional
    ? view.state.selection.map(changeSet).addRange(EditorSelection.cursor(head))
    : EditorSelection.cursor(head);
  view.dispatch({ changes: changeSet, selection, userEvent: insert.length || insideTab ? "input.indent" : "select.pointer", scrollIntoView: true });
  view.focus();
  event.preventDefault();
  return true;
}

export function indentationGuides(path: string) {
  const extension = path.split(".").at(-1)?.toLowerCase() ?? "";
  const marker = /^(py|pyw|sh|bash|zsh|rb|r|pl|pm|yaml|yml|toml|conf)$/.test(extension) ? "#"
    : /^(js|jsx|ts|tsx|mjs|cjs|go|c|h|cpp|hpp|cc|cs|java|rs|swift|kt|kts|scala|dart|php)$/.test(extension) ? "//"
    : /^(sql|lua|hs)$/.test(extension) ? "--" : "";
  const guides = StateField.define({
    create: state => buildGuides(state, marker),
    update(value, transaction) {
      return transaction.docChanged || transaction.startState.tabSize !== transaction.state.tabSize ? buildGuides(transaction.state, marker) : value;
    },
    provide: field => EditorView.decorations.from(field, value => value.decorations),
  });
  return [
    guides,
    EditorView.theme({ "&": { "--indent-guide-color": "#52616a66" } }),
    EditorView.domEventHandlers({ mousedown: (event, view) => clickGuide(event, view, view.state.field(guides).columns, marker) }),
  ];
}
