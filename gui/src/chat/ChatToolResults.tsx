import { useState } from "react";
import type { ChatToolResult } from "./chatTypes";

export function EditFileDiffCard({ newString, oldString }: { newString?: string; oldString?: string }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const oldLines = splitEditFileLines(oldString);
  const newLines = splitEditFileLines(newString);
  const canExpand = oldLines.length > 1 || newLines.length > 1;
  const visibleOldLines = isExpanded ? oldLines : oldLines.slice(0, 1);
  const visibleNewLines = isExpanded ? newLines : newLines.slice(0, 1);
  const toggleExpanded = () => {
    if (canExpand) setIsExpanded((current) => !current);
  };

  return (
    <div className={`chat-tool-edit-diff${isExpanded ? " is-expanded" : ""}`}>
      {visibleOldLines.map((line, index) => (
        <button
          aria-expanded={canExpand ? isExpanded : undefined}
          aria-label={canExpand ? (isExpanded ? "Collapse removed lines" : "Show all removed lines") : undefined}
          className="chat-tool-edit-line is-old"
          key={`old-${index}`}
          onClick={toggleExpanded}
          type="button"
        >
          {line || " "}
        </button>
      ))}
      {visibleNewLines.map((line, index) => (
        <button
          aria-expanded={canExpand ? isExpanded : undefined}
          aria-label={canExpand ? (isExpanded ? "Collapse added lines" : "Show all added lines") : undefined}
          className="chat-tool-edit-line is-new"
          key={`new-${index}`}
          onClick={toggleExpanded}
          type="button"
        >
          {line || " "}
        </button>
      ))}
    </div>
  );
}

function splitEditFileLines(value?: string): string[] {
  const normalized = (value ?? "").replace(/\r\n?/g, "\n");
  if (!normalized) return [];
  const lines = normalized.split("\n");
  if (lines.at(-1) === "") lines.pop();
  return lines;
}

export function ToolResultCard({ result, toolName }: { result: ChatToolResult; toolName: string }) {
  if (result.status === "error") return <ToolErrorResultCard result={result} />;
  if (toolName === "find") {
    return <CountedToolResultCard collapseLabel="find results" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "searchSkill") {
    return <CountedToolResultCard collapseLabel="skill matches" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "searchTools") {
    return <CountedToolResultCard collapseLabel="tool matches" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "listDir") {
    return <CountedToolResultCard collapseLabel="directory entries" plural="entries" result={result} singular="entry" />;
  }
  if (toolName === "deepResearch") {
    return <DeepResearchResultCard result={result} />;
  }
  if (toolName === "todoList") {
    return <TodoListResultCard result={result} />;
  }
  if (toolName === "researchStatus") {
    return <ResearchStatusResultCard result={result} />;
  }
  return <GenericToolResultCard result={result} />;
}

function ToolErrorResultCard({ result }: { result: ChatToolResult }) {
  const message = result.error?.trim() || result.output?.trim() || result.summary?.trim() || "The tool returned an error.";

  return (
    <div aria-label={`Error: ${message}`} className="chat-tool-result chat-tool-error-result is-error" role="alert">
      <span className="chat-tool-result-prefix">Error:</span>
      <span className="chat-tool-result-text">{message}</span>
    </div>
  );
}

function DeepResearchResultCard({ result }: { result: ChatToolResult }) {
  const status = result.researchStatus ?? "unknown";

  return (
    <div className={`chat-tool-result chat-research-status-result is-${result.status}`}>
      <span className="chat-research-status-prefix">Result:</span>
      <span className={`chat-research-status-value is-${status}`}>{status}</span>
      {result.jobId ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">jobId:</span>
          <span className="chat-research-status-value">{result.jobId}</span>
        </div>
      ) : null}
      {result.title ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">title:</span>
          <span className="chat-research-status-value">{result.title}</span>
        </div>
      ) : null}
    </div>
  );
}

function ResearchStatusResultCard({ result }: { result: ChatToolResult }) {
  const status = result.researchStatus ?? "unknown";

  return (
    <div className={`chat-tool-result chat-research-status-result is-${result.status}`}>
      <span className="chat-research-status-prefix">Result:</span>
      <span className={`chat-research-status-value is-${status}`}>{status}</span>
      {result.phase ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">phase:</span>
          <span className="chat-research-status-value">{result.phase}</span>
        </div>
      ) : null}
      {typeof result.round === "number" && typeof result.maxRounds === "number" ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">round:</span>
          <span className="chat-research-status-value">{result.round}/{result.maxRounds}</span>
        </div>
      ) : null}
    </div>
  );
}

function GenericToolResultCard({ result }: { result: ChatToolResult }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const output = (result.output ?? "").replace(/\r\n?/g, "\n");
  const firstLine = result.summary ?? (output.split("\n")[0] || "—");
  const displayedOutput = isExpanded ? output || "—" : firstLine;
  const toggleLabel = isExpanded ? "Collapse result" : "Show full result";

  return (
    <div className={`chat-tool-result is-${result.status}${isExpanded ? " is-expanded" : ""}`}>
      <button aria-expanded={isExpanded} aria-label={toggleLabel} className="chat-tool-result-toggle" onClick={() => setIsExpanded((current) => !current)} type="button">
        <span className="chat-tool-result-prefix">Result:</span>
        <span className="chat-tool-result-text">{displayedOutput}</span>
      </button>
    </div>
  );
}

function CountedToolResultCard({
  collapseLabel,
  plural,
  result,
  singular,
}: {
  collapseLabel: string;
  plural: string;
  result: ChatToolResult;
  singular: string;
}) {
  const [isExpanded, setIsExpanded] = useState(false);
  const outputItems = (result.output ?? "").replace(/\r\n?/g, "\n").split("\n").filter(Boolean);
  const items = result.items?.length ? result.items : outputItems;
  const count = typeof result.count === "number" ? result.count : items.length;

  if (!isExpanded) {
    return (
      <div className={`chat-tool-result is-${result.status}`}>
        <button aria-expanded={false} aria-label={`Show ${collapseLabel}`} className="chat-tool-result-toggle" onClick={() => setIsExpanded(true)} type="button">
          <span className="chat-tool-result-prefix">Result:</span>
          <span className="chat-tool-result-text">{count} {count === 1 ? singular : plural}</span>
        </button>
      </div>
    );
  }

  return (
    <div className={`chat-tool-result chat-tool-result-list is-${result.status} is-expanded`}>
      <div className="chat-tool-result-list-label">Result:</div>
      <div className="chat-tool-result-items">
        {items.map((item) => (
          <button aria-label={`Collapse ${collapseLabel}`} className="chat-tool-result-item" key={item} onClick={() => setIsExpanded(false)} type="button">
            {item}
          </button>
        ))}
      </div>
    </div>
  );
}

function TodoListResultCard({ result }: { result: ChatToolResult }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const items = result.todoItems?.length ? result.todoItems : parseTodoItems(result.output);
  const orderedItems = [...items.filter((item) => item.checked), ...items.filter((item) => !item.checked)];
  const count = typeof result.count === "number" ? result.count : items.length;
  const completed = typeof result.completed === "number" ? result.completed : items.filter((item) => item.checked).length;
  const summary = `${count} ${count === 1 ? "todo" : "todos"}, ${completed} completed`;

  if (!isExpanded) {
    return (
      <div className={`chat-tool-result is-${result.status}`}>
        <button aria-expanded={false} aria-label="Show todo list" className="chat-tool-result-toggle" onClick={() => setIsExpanded(true)} type="button">
          <span className="chat-tool-result-prefix">Result:</span>
          <span className="chat-tool-result-text">{summary}</span>
        </button>
      </div>
    );
  }

  return (
    <div className={`chat-tool-result chat-tool-result-list is-${result.status} is-expanded`}>
      <div className="chat-tool-result-list-label">Result:</div>
      <div className="chat-tool-result-items">
        {orderedItems.map((item) => (
          <button aria-label={`Collapse todo list: ${item.text}`} className="chat-tool-result-todo-item" key={`${item.checked}-${item.text}`} onClick={() => setIsExpanded(false)} type="button">
            <span aria-hidden="true" className={`chat-tool-result-todo-checkbox${item.checked ? " is-checked" : ""}`} />
            <span>{item.text}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function parseTodoItems(output?: string) {
  return (output ?? "")
    .replace(/\r\n?/g, "\n")
    .split("\n")
    .map((line) => line.match(/^\s*-\s*\[([ xX])\]\s*(.+?)\s*$/))
    .filter((match): match is RegExpMatchArray => Boolean(match))
    .map((match) => ({ checked: match[1].toLowerCase() === "x", text: match[2] }));
}
