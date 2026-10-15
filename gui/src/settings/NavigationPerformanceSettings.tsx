import { useState } from "react";
import { clearNavigationPerformance, downloadNavigationPerformance, navigationReport, navigationSummary, type NavigationSample } from "../chat/navigationPerformance";
import { copyText } from "../chat/chatClipboard";
import "./navigation-performance.css";

const ms = (value?: number) => value === undefined ? "—" : `${Math.round(value)} ms`;
const span = (sample: NavigationSample, phase: string) => sample.spans.find(entry => entry.phase === phase)?.durationMs;
export function NavigationPerformanceSettings({ query }: { query: string }) {
  const [report, setReport] = useState(navigationReport);
  const [copyStatus, setCopyStatus] = useState("");
  if (!"advanced diagnostics chat thread navigation performance latency benchmark".includes(query.trim().toLowerCase())) return null;
  return <section aria-label="Chat performance diagnostics" className="settings-models navigation-performance">
    <header className="settings-models-header"><h2>Chat performance</h2></header>
    <div className="settings-models-content">
      <p>Last 100 thread switches, recorded automatically on this device. Message text and attachments are excluded. Measurements survive reloads.</p>
      <p>GUI build: <code>{report.buildID}</code></p>
      <p>Ready measures two animation frames after the view commits: a paint opportunity, not completion of code highlighting. Request timings include browser scheduling and network time.</p>
      <div className="navigation-performance-actions">
        <button type="button" onClick={() => setReport(navigationReport())}>Refresh measurements</button>
        <button type="button" onClick={() => downloadNavigationPerformance()}>Export JSON</button>
        <button type="button" onClick={() => {
          void copyText(JSON.stringify(navigationReport(), null, 2)).then(() => setCopyStatus("Report copied"), () => setCopyStatus("Unable to copy report"));
        }}>Copy report</button>
        <button type="button" onClick={() => { clearNavigationPerformance(); setReport(navigationReport()); }}>Clear measurements</button>
        <span role="status">{copyStatus}</span>
      </div>
      {[true, false].map(cached => {
        const summary = navigationSummary(report.samples.filter(sample => sample.cached === cached && sample.buildID === report.buildID));
        return <p key={String(cached)}><strong>{cached ? "Cached" : "First load"} (current build)</strong>: {summary.count} completed switches · median {ms(summary.medianMs)} · p95 {ms(summary.p95Ms)} · max {ms(summary.maxMs)}</p>;
      })}
      {!report.longTasksSupported ? <p>This browser does not expose long-task measurements.</p> : null}
      {!report.samples.length ? <p>Open a few threads, then return here and refresh.</p> : <div className="navigation-performance-table"><table>
        <caption>Recent switches (milliseconds; full phase details are in the exported report)</caption>
        <thead><tr>{["Time / Thread", "Source", "Messages", "Commit", "Ready", "Headers", "JSON", "Git", "Long tasks", "Status"].map(label => <th key={label}>{label}</th>)}</tr></thead>
        <tbody>{report.samples.slice(-20).reverse().map(sample => <tr key={sample.id}>
          <td title={`${sample.chatID} · Build ${sample.buildID}`}>{new Date(sample.timestamp).toLocaleTimeString()} / {sample.chatID.slice(0, 8)}</td>
          <td>{sample.cached ? "Cache" : "Server"}</td><td>{sample.messageCount}</td><td>{ms(sample.commitMs)}</td><td>{ms(sample.paintOpportunityMs)}</td>
          <td>{ms(span(sample, "chat.headers"))}</td><td>{ms(span(sample, "chat.json"))}</td>
          <td>{ms(Math.max(span(sample, "git.branches") ?? 0, span(sample, "git.worktrees") ?? 0))}</td>
          <td>{ms(sample.longTasks.reduce((total, task) => total + task.durationMs, 0))}</td><td>{sample.outcome}</td>
        </tr>)}</tbody>
      </table></div>}
    </div>
  </section>;
}
