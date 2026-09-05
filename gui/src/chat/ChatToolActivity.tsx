import { useEffect, useRef, useState } from "react";
import { go } from "@codemirror/lang-go";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { EditorView as CodeMirrorView, lineNumbers } from "@codemirror/view";
import { tags } from "@lezer/highlight";
import type { ChatToolCall, ChatToolResult } from "./chatTypes";
import { CheckpointLabel } from "./ChatMessageParts";
import { HammerIcon } from "./ChatIcons";
import { EditFileDiffCard, ToolResultCard } from "./ChatToolResults";
import { toolCheckpoint } from "./chatMessageUtils";
import { serverEndpoint } from "../platform";

const MISSING_TOOL_INTENT_LABEL = "Intent missing";

const codeModeHighlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.controlKeyword, tags.modifier, tags.operatorKeyword], color: "#77ddd1" },
  { tag: [tags.typeName, tags.className, tags.namespace, tags.definition(tags.typeName)], color: "#7ee0d5" },
  { tag: [tags.function(tags.variableName), tags.labelName], color: "#e8aa76" },
  { tag: [tags.string, tags.special(tags.string)], color: "#e79be1" },
  { tag: [tags.number, tags.bool, tags.null], color: "#e7c15f" },
  { tag: tags.comment, color: "#7f8981" },
  { tag: [tags.propertyName, tags.variableName], color: "#e1e4db" },
  { tag: [tags.operator, tags.punctuation], color: "#eed85b" },
]);


function displayToolIntent(tool: ChatToolCall) {
  return tool.intent?.trim() || MISSING_TOOL_INTENT_LABEL;
}

function toolIntentClassName(tool: ChatToolCall) {
  return `chat-tool-intent${tool.intent?.trim() ? "" : " is-missing"}`;
}


export function ToolCallCard({ tool }: { tool: ChatToolCall }) {
  if (tool.name === "orchestrate") {
    return <OrchestrateToolCard tool={tool} />;
  }

  const status = tool.status ?? tool.result?.status ?? "running";
  const checkpoint = toolCheckpoint(tool);
  const isFind = tool.name === "find";
  const isCreatePlan = tool.name === "createPlan";
  const isEditPlan = tool.name === "editPlan";
  const isBuildPlan = tool.name === "buildPlan";
  const isAddTodo = tool.name === "addTodo";
  const isTodoList = tool.name === "todoList";
  const isCheckTodo = tool.name === "checkTodo";
  const isRemoveTodo = tool.name === "removeTodo";
  const isCheckPlan = tool.name === "checkPlan";
  const isDeletePlan = tool.name === "deletePlan";
  const isFetchWeb = tool.name === "fetchWeb";
  const isWebSearch = tool.name === "webSearch";
  const isSearchSkill = tool.name === "searchSkill";
  const isSearchTools = tool.name === "searchTools";
  const isListSubAgents = tool.name === "listSubAgents";
  const isResearchStatus = tool.name === "researchStatus";
  const isRename = tool.name === "editFile" && Boolean(tool.renameTo);
  const isDelete = tool.name === "editFile" && Boolean(tool.delete);
  const isInlineTool = tool.name === "shell" || tool.name === "readFile" || isFind || tool.name === "listDir" || tool.name === "tree" || tool.name === "editFile" || tool.name === "loadSkill" || isSearchSkill || isSearchTools || isListSubAgents || isResearchStatus || tool.name === "deepResearch" || tool.name === "docsRetrieval" || isCreatePlan || isEditPlan || isBuildPlan || isAddTodo || isTodoList || isCheckTodo || isRemoveTodo || isCheckPlan || isDeletePlan || isFetchWeb || isWebSearch;
  const isDangerousArgument = isDelete || isRemoveTodo || isDeletePlan;
  const inlineCommand = isListSubAgents
    ? ""
    : tool.name === "editFile" && tool.renameTo
    ? `${tool.input ?? ""} → ${tool.renameTo}`
    : tool.input;
  const toolParameters = isFind || isCreatePlan || isAddTodo || isFetchWeb || isWebSearch
    ? tool.parameters ?? (isFind && tool.input ? [{ label: "pattern", value: tool.input }] : [])
    : [];
  const suppressResult = status === "success" && (
    isListSubAgents ||
    tool.name === "readFile" ||
    (tool.name === "editFile" && status === "success") ||
    (tool.name === "loadSkill" && status === "success") ||
    (tool.name === "docsRetrieval" && status === "success") ||
    (isCreatePlan && status === "success") ||
    (isEditPlan && status === "success") ||
    (isBuildPlan && status === "success") ||
    (isAddTodo && status === "success") ||
    (isCheckTodo && status === "success") ||
    (isRemoveTodo && status === "success") ||
    (isCheckPlan && status === "success") ||
    (isDeletePlan && status === "success") ||
    (isFetchWeb && status === "success")
  );
  const [isOpen, setIsOpen] = useState(() => tool.defaultOpen ?? false);

  return (
    <div className={`chat-tool-card is-${status}`} data-checkpoint={checkpoint?.label}>
      {checkpoint ? <CheckpointLabel className="chat-tool-checkpoint-label" label={checkpoint.label} /> : null}
      <details aria-busy={status === "running"} onToggle={(event) => setIsOpen(event.currentTarget.open)} open={isOpen}>
        <summary className="chat-tool-summary">
          <i aria-hidden="true" className="chat-tool-status-dot" />
          <HammerIcon />
          <span className={toolIntentClassName(tool)} title={displayToolIntent(tool)}>{displayToolIntent(tool)}</span>
          <svg aria-hidden="true" className="chat-tool-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="chat-tool-body">
          <div className="chat-tool-execution">
            <div className="chat-tool-name-row">
              {isInlineTool ? (
                <>
                  <strong className="chat-tool-name chat-tool-label">Tool:</strong>
                  <strong className="chat-tool-name">{tool.name}</strong>
                  {isFind ? <span className="chat-tool-command">{tool.mode ?? "text"}</span> : inlineCommand ? <span className={`chat-tool-command${isDangerousArgument ? " is-delete" : ""}`}>{inlineCommand}</span> : null}
                </>
              ) : (
                <strong className="chat-tool-name">{tool.name}</strong>
              )}
            </div>
            {tool.input && !isInlineTool ? (
              <div className="chat-tool-input">
                <span aria-hidden="true" className="chat-tool-prompt">$</span>
                <pre><code>{tool.input}</code></pre>
              </div>
            ) : null}
            {isCheckPlan && tool.full ? (
              <div className="chat-tool-parameters">
                <div className="chat-tool-parameter">
                  <span className="chat-tool-parameter-value">full</span>
                </div>
              </div>
            ) : null}
            {(isFind || isCreatePlan || isAddTodo || isFetchWeb || isWebSearch) && toolParameters.length ? (
              <div className="chat-tool-parameters">
                {toolParameters.map((parameter) => (
                  <div className={`chat-tool-parameter${isAddTodo && parameter.label === "todo" ? " is-todo" : ""}`} key={`${parameter.label}-${parameter.value}`}>
                    {isAddTodo && parameter.label === "todo" ? (
                      <>
                        <span aria-hidden="true" className="chat-tool-todo-checkbox" />
                        <span className="chat-tool-parameter-value">{parameter.value}</span>
                      </>
                    ) : (
                      <>
                        <span className="chat-tool-parameter-label">{parameter.label}:</span>
                        <span className="chat-tool-parameter-value"> {parameter.value}</span>
                      </>
                    )}
                  </div>
                ))}
              </div>
            ) : null}
            {(tool.name === "editFile" || isEditPlan) && !isRename && !isDelete && (tool.oldString !== undefined || tool.newString !== undefined) ? (
              <EditFileDiffCard newString={tool.newString} oldString={tool.oldString} />
            ) : null}
            {tool.result ? (
              suppressResult ? null : <ToolResultCard result={tool.result} toolName={tool.name} />
            ) : null}
          </div>
        </div>
      </details>
    </div>
  );
}

function OrchestrateToolCard({ tool }: { tool: ChatToolCall }) {
  const status = tool.status ?? tool.result?.status ?? "running";
  const checkpoint = toolCheckpoint(tool);
  const [isOpen, setIsOpen] = useState(() => tool.defaultOpen ?? false);
  const [isSourceOpen, setIsSourceOpen] = useState(false);
  const result = tool.result;
  const sdkCalls = result?.sdkCalls;
  const duration = formatOrchestrateDuration(result?.durationMs);
  const source = tool.input?.replace(/\r\n?/g, "\n") ?? "";
  const sourcePanelID = `${tool.id}-source`;

  return (
    <div className={`chat-tool-card chat-code-mode-card is-${status}`} data-checkpoint={checkpoint?.label}>
      {checkpoint ? <CheckpointLabel className="chat-tool-checkpoint-label" label={checkpoint.label} /> : null}
      <details aria-busy={status === "running"} onToggle={(event) => setIsOpen(event.currentTarget.open)} open={isOpen}>
        <summary className="chat-tool-summary chat-code-mode-summary">
          <i aria-hidden="true" className="chat-tool-status-dot" />
          <HammerIcon />
          <span className="chat-code-mode-label">CODE</span>
          <span className={toolIntentClassName(tool)} title={displayToolIntent(tool)}>{displayToolIntent(tool)}</span>
          <svg aria-hidden="true" className="chat-tool-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="chat-tool-body chat-code-mode-body">
          <div className="chat-code-mode-row chat-code-mode-tool-row">
            <span className="chat-code-mode-row-label">Tool:</span>
            <strong className="chat-code-mode-row-value">orchestrate</strong>
          </div>
          <div className="chat-code-mode-row chat-code-mode-sdk-row">
            <span className="chat-code-mode-row-label">SDK calls</span>
            <span className="chat-code-mode-row-value">{sdkCalls ?? "—"}</span>
            {duration ? <span className="chat-code-mode-row-meta">{duration}</span> : null}
          </div>
          <button
            aria-controls={sourcePanelID}
            aria-expanded={isSourceOpen}
            className="chat-code-mode-source-toggle"
            disabled={!source}
            onClick={() => setIsSourceOpen((current) => !current)}
            type="button"
          >
            {isSourceOpen ? "Hide source" : "Show source"}
            <svg aria-hidden="true" className="chat-code-mode-source-chevron" viewBox="0 0 24 24">
              <path d="m7 10 5 5 5-5" />
            </svg>
          </button>
          {isSourceOpen ? (
            <div aria-label="Orchestration source" className="chat-code-mode-source" id={sourcePanelID}>
              <CodeModeSource source={source} />
            </div>
          ) : null}
          {result ? <OrchestrateResult result={result} status={status} /> : (
            <div className="chat-code-mode-pending">Waiting for the daemon result…</div>
          )}
        </div>
      </details>
    </div>
  );
}

function CodeModeSource({ source }: { source: string }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [displaySource, setDisplaySource] = useState(source);

  useEffect(() => {
    let cancelled = false;
    setDisplaySource(source);
    void formatGoDisplaySource(source).then((formatted) => {
      if (!cancelled) setDisplaySource(formatted);
    });
    return () => {
      cancelled = true;
    };
  }, [source]);

  useEffect(() => {
    const parent = hostRef.current;
    if (!parent) return;

    const view = new CodeMirrorView({
      parent,
      state: EditorState.create({
        doc: displaySource,
        extensions: [
          lineNumbers(),
          CodeMirrorView.lineWrapping,
          EditorState.readOnly.of(true),
          CodeMirrorView.editable.of(false),
          syntaxHighlighting(codeModeHighlightStyle, { fallback: true }),
          go(),
          CodeMirrorView.theme(
            {
              "&": { background: "transparent", color: "#dce6e9", height: "100%" },
              ".cm-scroller": { fontFamily: '\"Geist Mono\", \"SFMono-Regular\", \"Cascadia Code\", ui-monospace, monospace', overflow: "auto" },
              ".cm-content": { caretColor: "transparent", padding: "10px 0 11px" },
              ".cm-line": { padding: "0 12px 0 8px" },
              ".cm-gutters": { background: "transparent", border: "none", color: "#52616a", minWidth: "48px" },
              ".cm-gutterElement": { padding: "0 9px 0 8px" },
              ".cm-activeLine, .cm-activeLineGutter": { background: "transparent" },
              "&.cm-focused": { outline: "none" },
              ".cm-selectionBackground, ::selection": { background: "transparent !important" },
            },
            { dark: true },
          ),
        ],
      }),
    });

    return () => view.destroy();
  }, [displaySource]);

  return <div className="chat-code-mode-source-editor"><div ref={hostRef} /></div>;
}

function OrchestrateResult({ result, status }: { result: ChatToolResult; status: NonNullable<ChatToolCall["status"]> }) {
  const output = (result.output ?? "").replace(/\r\n?/g, "\n").trim();
  const error = result.error?.trim();
  const body = error || output || result.summary || (status === "success" ? "Script completed without printed output." : "No result output.");

  return (
    <div className={`chat-code-mode-result is-${result.status}`}>
      <div className="chat-code-mode-result-heading">
        <span className="chat-code-mode-result-label">Result</span>
        {result.truncated ? <span className="chat-code-mode-result-truncated">truncated</span> : null}
      </div>
      <pre className={error ? "is-error" : undefined}>{body}</pre>
    </div>
  );
}


async function formatGoDisplaySource(source: string): Promise<string> {
  if (!source.trim()) return source;
  try {
    const endpoint = await serverEndpoint("/__solomon/format-go");
    const response = await fetch(endpoint, {
      body: JSON.stringify({ source }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    if (!response.ok) return source;
    const payload: unknown = await response.json();
    const record = payload && typeof payload === "object" && !Array.isArray(payload) ? payload as { source?: unknown } : {};
    return typeof record.source === "string" && record.source ? record.source : source;
  } catch {
    return source;
  }
}
function formatOrchestrateDuration(durationMs?: number) {
  if (durationMs === undefined || !Number.isFinite(durationMs) || durationMs < 0) return "";
  const seconds = durationMs / 1000;
  return `${seconds.toFixed(seconds >= 10 ? 1 : 2).replace(/0+$/, "").replace(/\.$/, "")}s`;
}

export function SubagentCard({ onOpenSubagent, onStopTool, tool }: { onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; tool: ChatToolCall }) {
	const status = subagentStatus(tool);
	const displayStatus = subagentDisplayStatus(tool);
	const checkpoint = toolCheckpoint(tool);
	const canOpen = Boolean(onOpenSubagent);

  return (
    <section
      aria-label="Open subagent chat"
		className={`chat-subagent-card is-${displayStatus}`}
		data-checkpoint={checkpoint?.label}
		onClick={() => {
			onOpenSubagent?.(tool);
		}}
		onKeyDown={(event) => {
			if (!canOpen || event.target !== event.currentTarget || (event.key !== "Enter" && event.key !== " ")) return;
			event.preventDefault();
			onOpenSubagent?.(tool);
		}}
		role={canOpen ? "button" : undefined}
		tabIndex={canOpen ? 0 : -1}
    >
      {checkpoint ? <CheckpointLabel className="chat-tool-checkpoint-label" label={checkpoint.label} /> : null}
      <div className="chat-subagent-content">
        <div className="chat-subagent-heading">
          <i aria-hidden="true" className="chat-subagent-status-dot" />
          <span className="chat-subagent-title">Subagent</span>
          {tool.sync ? <span className="chat-subagent-mode">sync</span> : null}
          {subagentIsActive(status) ? (
            <button
              aria-label="Stop subagent"
              className="chat-subagent-stop"
              onClick={(event) => {
                event.stopPropagation();
                onStopTool?.(tool.id);
              }}
              title="Stop subagent"
              type="button"
            >
              Stop
            </button>
          ) : null}
        </div>
        {tool.input ? <p className="chat-subagent-task">{tool.input}</p> : null}
      </div>
    </section>
  );
}


export function subagentStatus(tool: ChatToolCall): string {
  return tool.result?.subchatStatus ?? tool.status ?? tool.result?.status ?? "running";
}

export function subagentDisplayStatus(tool: ChatToolCall): string {
	const status = subagentStatus(tool);
	if (status === "done") return "success";
	if (status === "cancelled" || status === "failed" || status === "interrupted" || status === "paused") return "error";
	return status;
}

export function subagentIsActive(status: string): boolean {
  return status === "running" || status === "queued";
}
