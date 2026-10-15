import { build } from "vite";
import { chromium } from "playwright-core";
import { createServer } from "node:http";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const args = process.argv.slice(2);
const option = (key, fallback) => args.includes(key) ? args[args.indexOf(key) + 1] : fallback;
const sizes = option("--messages", "20,100,300").split(",").map(Number);
const repeats = Number(option("--repeats", "6"));
const serverDelayMs = Number(option("--server-delay", "50"));
const output = resolve(option("--output", "chat-navigation-benchmark.json"));
if (sizes.some(size => !Number.isInteger(size) || size < 1 || size > 2000) || !Number.isInteger(repeats) || repeats < 1 || repeats > 100 || !Number.isFinite(serverDelayMs) || serverDelayMs < 0) throw new Error("Invalid benchmark arguments");
const directory = await mkdtemp(join(tmpdir(), "solomon-chat-benchmark-"));
const sourceDirectory = await mkdtemp(join(root, ".navigation-benchmark-"));
const input = join(sourceDirectory, "index.html");
const outDir = join(directory, "dist");
let browser, server;
const fixture = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import { ChatView } from '/src/chat/ChatView.tsx';
import { useChatRuntime } from '/src/chat/useChatRuntime.ts';
import { clearChatStore } from '/src/chat/chatStore.ts';
import { navigationReport, clearNavigationPerformance } from '/src/chat/navigationPerformance.ts';
import '/src/styles.css';
import '/src/home/welcome.css';
window.fixtureChats = {};
window.fetch = async url => {
  await new Promise(resolve => setTimeout(resolve, ${serverDelayMs}));
  const path = new URL(url, location.href).pathname;
  const id = path.split('/').at(-1);
  const data = window.fixtureChats[id] ?? (path.endsWith('/branches') ? {isRepo:true,current:'main',branches:['main']} : path.endsWith('/worktrees') ? {worktrees:[]} : {projects:[], chat:{startToolCallsCollapsed:false,autoCloseToolCalls:false,startReasoningCollapsed:false},current:{provider:'',model:''},providers:[],recent:[]});
  return new Response(JSON.stringify(data), {headers:{'Content-Type':'application/json'}});
};
function Fixture() {
  const runtime = useChatRuntime();
  window.openChat = id => { void runtime.openProjectChat({id:'benchmark',name:'Benchmark',path:'/tmp/benchmark'}, id); };
  window.configure = (size, key) => {
    runtime.clearSelection(); clearChatStore(); clearNavigationPerformance();
    window.fixtureChats = {};
    for (const suffix of ['a','b']) {
      const id = key + '-' + suffix;
      const messages = Array.from({length:size}, (_,index) => ({id:String(index),role:index%2?'assistant':'user',content:index%2?
        '## Response ' + index + '\\n\\n' + 'A paragraph with **bold**, \`inline code\`, and [a link](https://example.com). '.repeat(12) + '\\n\\n~~~js\\nconst result = { value: 42 };\\nconsole.log(result);\\n~~~\\n\\n| Key | Value |\\n| --- | --- |\\n| result | 42 |' : 'Please explain step ' + index}));
      window.fixtureChats[id] = {id,title:id,mode:'agent',status:'done',messages};
    }
  };
  window.report = navigationReport;
  return runtime.selectedChat ? <ChatView key={runtime.selectedChat.id} chat={runtime.selectedChat} isStreaming={false}
    onSend={runtime.sendMessage} onDeleteMessage={runtime.deleteMessage} onStopTool={runtime.stopTool} onStopStreaming={runtime.stopChat} /> : <p>Waiting</p>;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
try {
  await writeFile(input, '<div id="root" class="app-shell"></div><script type="module" src="/__benchmark.tsx"></script>');
  const bundle = await build({ root, configFile: false, logLevel: "error",
    plugins: [{ name: "benchmark-fixture", resolveId: id => id === "/__benchmark.tsx" ? id : undefined, load: id => id === "/__benchmark.tsx" ? fixture : undefined }],
    define: { __SOLOMON_GUI_BUILD_ID__: JSON.stringify(`benchmark-${new Date().toISOString()}`) },
    esbuild: { jsx: "automatic" }, build: { outDir, emptyOutDir: true, rollupOptions: { input }, minify: true },
  });
  const html = bundle.output.find(item => item.type === "asset" && item.fileName.endsWith(".html"));
  if (!html) throw new Error("Benchmark HTML was not emitted");
  server = createServer(async (request, response) => {
    try {
      const pathname = new URL(request.url, "http://localhost").pathname;
      const file = pathname === "/" ? join(outDir, html.fileName) : join(outDir, pathname);
      if (!resolve(file).startsWith(resolve(outDir) + "/")) { response.writeHead(403).end(); return; }
      response.setHeader("Content-Type", file.endsWith(".js") ? "text/javascript" : file.endsWith(".css") ? "text/css" : "text/html");
      response.end(await readFile(file));
    } catch { response.writeHead(404).end(); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
  if (!executablePath) throw new Error("Install Chromium or set SOLOMON_TEST_BROWSER");
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  page.setDefaultTimeout(60000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.goto(`http://127.0.0.1:${server.address().port}/`);
  await page.waitForFunction(() => typeof window.configure === "function");
  const cases = [];
  for (const size of sizes) {
    const key = `messages-${size}`;
    await page.evaluate(({ size, key }) => window.configure(size, key), { size, key });
    for (let index = 0; index < repeats + 2; index++) {
      const id = `${key}-${index % 2 ? "b" : "a"}`;
      const before = await page.evaluate(() => window.report().samples.length);
      await page.evaluate(id => window.openChat(id), id);
      await page.waitForFunction(before => {
        const samples = window.report().samples;
        const sample = samples.at(-1);
        return samples.length > before && sample?.paintOpportunityMs !== undefined && sample.spans.some(span => span.phase === 'chat.total') && sample.spans.some(span => span.phase === 'git.worktrees');
      }, before);
      // Include delayed syntax highlighting and long-task observer delivery.
      await page.waitForTimeout(500);
    }
    const report = await page.evaluate(() => window.report());
    const summary = cached => {
      const values = report.samples.filter(sample => sample.cached === cached && sample.outcome === "ready").map(sample => sample.paintOpportunityMs).sort((a, b) => a - b);
      return { count: values.length, medianMs: values[Math.ceil(values.length * .5) - 1], p95Ms: values[Math.ceil(values.length * .95) - 1], maxMs: values.at(-1) };
    };
    cases.push({ messages: size, firstLoad: summary(false), cached: summary(true), report });
    console.log(`${size} messages: first load median ${summary(false).medianMs} ms; cached median ${summary(true).medianMs} ms, p95 ${summary(true).p95Ms} ms`);
  }
  if (errors.length) throw new Error(errors.join("\n"));
  const report = { schemaVersion: 1, benchmark: "production React build, synthetic Markdown/code/table histories, real ChatView + useChatRuntime", browser: browser.version(), viewport: { width: 1280, height: 800 }, serverDelayMs, repeats, generatedAt: new Date().toISOString(), cases };
  await writeFile(output, JSON.stringify(report, null, 2));
  console.log(`Report: ${output}`);
} finally {
  if (browser) await browser.close();
  if (server) await new Promise(resolve => server.close(resolve));
  await rm(directory, { recursive: true, force: true });
  await rm(sourceDirectory, { recursive: true, force: true });
}
