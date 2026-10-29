import { replaceNext, replaceAll, selectMatches, closeSearchPanel, findNext, findPrevious, getSearchQuery, SearchQuery, setSearchQuery } from "@codemirror/search";
import { EditorView, type ViewUpdate } from "@codemirror/view";

export function createSearchPanel(view: EditorView) {
  const dom = document.createElement("div");
  dom.className = "editor-find";
  const firstRow = document.createElement("div");
  firstRow.className = "editor-find-row";
  dom.append(firstRow);
  const replacement = document.createElement("input");
  replacement.placeholder = "Replace";
  replacement.setAttribute("aria-label", "Replace with");
  replacement.value = getSearchQuery(view.state).replace;
  const field = document.createElement("div");
  field.className = "editor-find-field";
  const input = document.createElement("input");
  input.placeholder = "Find";
  input.setAttribute("aria-label", "Find in file");
  input.setAttribute("main-field", "true");
  input.value = getSearchQuery(view.state).search;
  field.append(input);
  const toggles: HTMLButtonElement[] = [];
  const button = (label: string, title: string, action: () => void, parent = firstRow) => {
    const el = document.createElement("button");
    el.type = "button";
    el.textContent = label;
    el.title = title;
    el.setAttribute("aria-label", title);
    el.onclick = action;
    parent.append(el);
    return el;
  };
  const updateQuery = () => {
    const q = getSearchQuery(view.state);
    view.dispatch({ effects: setSearchQuery.of(new SearchQuery({ search: input.value, replace: replacement.value, caseSensitive: q.caseSensitive, wholeWord: q.wholeWord, regexp: q.regexp })) });
  };
  input.oninput = updateQuery;
  replacement.oninput = updateQuery;
  for (const [label, title, property] of [["Aa", "Maiuscole/minuscole", "caseSensitive"], ["ab", "Parola intera", "wholeWord"], [".*", "Espressione regolare", "regexp"]] as const) {
    toggles.push(button(label, title, () => {
      const q = getSearchQuery(view.state);
      view.dispatch({ effects: setSearchQuery.of(new SearchQuery({ search: input.value, replace: replacement.value, caseSensitive: q.caseSensitive, wholeWord: q.wholeWord, regexp: q.regexp, [property]: !q[property] })) });
    }, field));
  }
  firstRow.append(field);
  const count = document.createElement("span");
  count.className = "editor-find-count";
  count.setAttribute("aria-live", "polite");
  firstRow.append(count);
  button("↑", "Precedente (Maiusc+Invio)", () => { findPrevious(view); });
  button("↓", "Successivo (Invio)", () => { findNext(view); });
  button("☷", "Seleziona tutte le occorrenze", () => { selectMatches(view); view.focus(); });
  button("×", "Chiudi (Esc)", () => { closeSearchPanel(view); view.focus(); });
  const secondRow = document.createElement("div");
  secondRow.className = "editor-find-row editor-find-replace";
  const replaceField = document.createElement("div");
  replaceField.className = "editor-find-field";
  replaceField.append(replacement);
  secondRow.append(replaceField);
  button("↵", "Sostituisci", () => { replaceNext(view); }, secondRow);
  button("⇉", "Sostituisci tutto", () => { replaceAll(view); }, secondRow);

  dom.append(secondRow);
  const expand = button("", "Mostra/nascondi sostituzione", () => {
    secondRow.hidden = !secondRow.hidden;
    expand.setAttribute("aria-expanded", String(!secondRow.hidden));
  }, dom);
  expand.className = "editor-find-expand";
  expand.setAttribute("aria-expanded", "true");
  const icons: Record<string, string> = {
    "↑": '<path d="m5 10 7-7 7 7M12 3v18"/>',
    "↓": '<path d="m5 14 7 7 7-7M12 3v18"/>',
    "×": '<path d="m6 6 12 12M18 6 6 18"/>',
    "☷": '<path d="M8 6h13M8 12h13M8 18h13M3 5v14m-2-2 2 2 2-2"/>',
    "↵": '<path d="M5 3h6m-3 0v5m-4 3h12v10H4zM18 5l3 3-3 3m-4-3h7M7 15h6m-6 3h4"/>',
    "⇉": '<path d="M3 3h6m-3 0v5m5-5h6m-3 0v5M2 12h14v9H2zM6 15v3m4-3v3m8-13 3 3-3 3m-3-3h6M19 15v6"/>',
    "": '<path d="m8 10 4 4 4-4"/>',
  };
  for (const el of dom.querySelectorAll("button")) {
    const path = icons[el.textContent ?? ""];
    if (path) el.innerHTML = `<svg viewBox="0 0 24 24" aria-hidden="true">${path}</svg>`;
  }
  toggles[1].style.textDecoration = "underline";
  dom.onkeydown = event => {
    if (event.key === "Escape") { event.preventDefault(); closeSearchPanel(view); view.focus(); }
    if (event.key === "Enter") { event.preventDefault(); event.target === replacement ? replaceNext(view) : event.shiftKey ? findPrevious(view) : findNext(view); }
  };
  const sync = () => {
    const q = getSearchQuery(view.state);
    if (input.value !== q.search) input.value = q.search;
    if (replacement.value !== q.replace) replacement.value = q.replace;
    [q.caseSensitive, q.wholeWord, q.regexp].forEach((active, i) => toggles[i].setAttribute("aria-pressed", String(active)));
    let total = 0, current = 0;
    if (q.valid && q.search) {
      const cursor = q.getCursor(view.state);
      for (let match = cursor.next(); !match.done; match = cursor.next()) {
        total++;
        if (match.value.from === view.state.selection.main.from && match.value.to === view.state.selection.main.to) current = total;
      }
    }
    count.textContent = !q.search ? "" : !q.valid ? "Regex non valida" : total ? `${current || '–'} di ${total}` : "Nessun risultato";
  };
  sync();
  return { dom, top: true, mount() { input.focus(); input.select(); }, update(_update: ViewUpdate) { sync(); } };
}
