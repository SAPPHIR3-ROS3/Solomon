import { useEffect, useState } from "react";
import { fetchProjectResearchReport, type Project, type ProjectResearch } from "../projects/projects";

type ResearchReportViewProps = {
  bottomInset?: number;
  project: Project;
  research: ProjectResearch;
};

const REPORT_SCROLL_FADE_MARKUP = `<div aria-hidden="true" class="solomon-report-scroll-fade solomon-report-scroll-fade-top"></div><div aria-hidden="true" class="solomon-report-scroll-fade solomon-report-scroll-fade-bottom"></div>`;

const REPORT_SCROLLBAR_STYLE = `<style id="solomon-report-scrollbar">
html { background: #061c3b; color-scheme: dark; overscroll-behavior: none; scrollbar-width: none; }
body { background: #061c3b !important; color: #e7effa !important; overscroll-behavior: none; padding-bottom: 56px !important; scrollbar-width: none; }
body :is(h1, h2, h3, h4, h5, h6, p, li, dt, dd, th, td, summary, figcaption, small, strong, em, code) { color: #e7effa; }
body a { color: #8dbdff; }
html::-webkit-scrollbar, body::-webkit-scrollbar { display: none; }
.solomon-report-scroll-fade { height: 36px; inset-inline: 0; pointer-events: none; position: fixed; z-index: 2147483647; }
.solomon-report-scroll-fade-top { background: linear-gradient(to bottom, #061c3b 0%, rgb(6 28 59 / 0%) 100%); top: 0; }
.solomon-report-scroll-fade-bottom { background: linear-gradient(to top, #061c3b 0%, rgb(6 28 59 / 0%) 100%); bottom: 0; }
</style>`;

function styleResearchReport(html: string) {
  const styled = /<\/head>/i.test(html) ? html.replace(/<\/head>/i, `${REPORT_SCROLLBAR_STYLE}</head>`) : `${REPORT_SCROLLBAR_STYLE}${html}`;
  if (/solomon-report-scroll-fade/.test(html)) return styled;
  return /<\/body>/i.test(styled) ? styled.replace(/<\/body>/i, `${REPORT_SCROLL_FADE_MARKUP}</body>`) : `${styled}${REPORT_SCROLL_FADE_MARKUP}`;
}

export function ResearchReportView({ bottomInset = 0, project, research }: ResearchReportViewProps) {
  const [report, setReport] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setReport("");
    setError("");
    void fetchProjectResearchReport(project.id, research.id)
      .then((html) => { if (!cancelled) setReport(html); })
      .catch(() => { if (!cancelled) setError("Unable to load this deep research report."); });
    return () => { cancelled = true; };
  }, [project.id, research.id]);

  return (
    <section aria-label={`Deep research: ${research.title}`} className="research-report-view" style={{ bottom: Math.max(0, bottomInset) }}>
      {error ? <p className="research-report-message" role="status">{error}</p> : null}
      {!error && !report ? <p className="research-report-message">Loading deep research…</p> : null}
      {report ? <iframe className="research-report-frame" sandbox="allow-popups allow-popups-to-escape-sandbox allow-same-origin" srcDoc={styleResearchReport(report)} title={research.title} /> : null}
    </section>
  );
}
