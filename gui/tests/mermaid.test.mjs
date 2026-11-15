import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright-core';
import { launchDesktopWebKit } from '../scripts/webkit-driver.mjs';

test('renders diagrams and recovers from invalid streaming source', async () => {
  const fixture = `import React from 'react';
    import {createRoot} from 'react-dom/client';
    import {flushSync} from 'react-dom';
    import {MarkdownContent} from '/src/chat/MarkdownContent.tsx';
    import '/src/theme/themes.css';
    import {diagramSize, DiagramUnitContext} from '/src/chat/diagramSizing.ts';
    window.diagramSize=diagramSize;
    import {themeDiagram} from '/src/chat/diagramTheme.ts';
    window.themeDiagram=themeDiagram;
    window.runtime={ClipboardSetText:async text=>{window.copied=text;return true;}};
    const root=createRoot(document.getElementById('root'));
    window.show=(content,isStreaming=true,unit=600)=>flushSync(()=>root.render(<DiagramUnitContext.Provider value={unit}><MarkdownContent content={content} isStreaming={isStreaming}/></DiagramUnitContext.Provider>));`;
  const cacheDir = await mkdtemp(join(tmpdir(), 'solomon-mermaid-'));
  const server = await createServer({
    cacheDir,
    root: fileURLToPath(new URL('..', import.meta.url)), configFile: false,
    optimizeDeps: { noDiscovery: true, include: ['react', 'react-dom/client', 'react-dom', 'react/jsx-runtime', 'react/jsx-dev-runtime', 'react-markdown', 'remark-gfm', 'remark-math', 'rehype-katex', 'katex', '@shikijs/core', '@shikijs/engine-oniguruma', '@shikijs/engine-oniguruma/wasm-inlined', 'mermaid', '@mermaid-js/mermaid-zenuml'] },
    esbuild: { jsx: 'automatic' }, server: { host: '127.0.0.1', port: 0, hmr: false },
    plugins: [{
      name: 'mermaid-fixture',
      resolveId: id => id === '/fixture.tsx' ? id : undefined,
      load: id => id === '/fixture.tsx' ? fixture : undefined,
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.url !== '/fixture') return next();
          res.setHeader('Content-Type', 'text/html');
          res.end('<!doctype html><div id="root"></div><script>window.errors=[];window.addEventListener("error",e=>window.errors.push(e.message));window.addEventListener("unhandledrejection",e=>window.errors.push(String(e.reason)));</script><script type="module" src="/fixture.tsx"></script>');
        });
      },
    }],
  });
  let browser;
  try {
    await server.listen();
    browser = process.env.SOLOMON_TEST_ENGINE === 'webkitgtk' ? await launchDesktopWebKit() : await chromium.launch({ executablePath: process.env.SOLOMON_TEST_BROWSER || '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] });
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
    await page.waitForFunction(() => typeof window.show === 'function' || window.errors?.length);
    assert.deepEqual(await page.evaluate(() => window.errors), []);
    const show = code => page.evaluate(content => window.show(content), '```mermaid\n' + code + '\n```');
    await show('flowchart TD\nA[Il tool termina] --> B[Ricevi output]\nB --> C{Output comprimibile?}');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview img')?.naturalWidth > 0 || document.querySelector('.chat-mermaid-status')?.title);
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    assert.match(decodeURIComponent(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').getAttribute('src'))), /Ricevi/);
    assert.equal(await page.evaluate(() => document.querySelector('pre')), null);
    await page.evaluate(() => document.querySelector('button[aria-label="Copy SVG"]').click());
    await page.waitForFunction(() => window.copied?.startsWith('<svg'));
    assert.match(await page.evaluate(() => window.copied), /#0d3566/);
    assert.match(await page.evaluate(() => window.copied), /#3b8fd1/);
    await page.evaluate(() => [...document.querySelectorAll('.chat-mermaid-switch button')].find(button => button.textContent === 'Source').click());
    await page.waitForFunction(() => !!document.querySelector('pre'));
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-preview')), null);
    await page.evaluate(() => document.querySelector('button[aria-label="Copy text"]').click());
    await page.waitForFunction(() => window.copied?.startsWith('flowchart TD'));
    assert.match(await page.evaluate(() => window.copied), /Output comprimibile/);
    await page.evaluate(() => document.querySelector('.chat-mermaid-switch button').click());
    const previousImage = await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').src);
    await show('not a valid diagram');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'true');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').src), previousImage);
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-status')?.textContent.includes('Preview unavailable'));
    assert.equal(await page.evaluate(() => document.querySelector('button[aria-label="Copy SVG"]').disabled), true);
    await page.evaluate(() => [...document.querySelectorAll('.chat-mermaid-switch button')].find(button => button.textContent === 'Source').click());
    await page.waitForFunction(() => !!document.querySelector('pre'));
    assert.match(await page.evaluate(() => document.querySelector('pre').innerText), /not a valid diagram/);
    await page.evaluate(() => document.querySelector('.chat-mermaid-switch button').click());
    await show('sequenceDiagram\nAlice->>Bob: Hello');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false' && !document.querySelector('.chat-mermaid-status')?.title);
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview img')?.naturalWidth > 0 || document.querySelector('.chat-mermaid-status')?.title);
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    assert.match(decodeURIComponent(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').getAttribute('src'))), /Hello/);
    await page.evaluate(() => window.show('```js\nconst x = 1;\n```'));
    await page.waitForFunction(() => !document.querySelector('.chat-mermaid-preview'));
    await show('sequenceDiagram\nAlice->>Bob: Hello');
    await page.waitForFunction(() => !!document.querySelector('.chat-mermaid-preview'));
    const restored = await page.evaluate(() => {
      const preview = document.querySelector('.chat-mermaid-preview');
      return {
        pending: preview.getAttribute('aria-busy'),
        animation: getComputedStyle(preview.querySelector('img')).animationName,
        transition: getComputedStyle(preview).transitionProperty,
        height: preview.style.height,
      };
    });
    assert.deepEqual(restored, { pending: 'false', animation: 'none', transition: 'none', height: '' });
    await show('zenuml\n title Accesso\n Utente->Server.accedi() {\n Server->Database.verifica()\n }');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview img')?.naturalWidth > 0);
    const zenSVG = decodeURIComponent(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').src));
    assert.match(zenSVG, /Accesso/);
    assert.match(zenSVG, /Database/);
    const zenColors = await page.evaluate(() => {
      const svg = decodeURIComponent(document.querySelector('.chat-mermaid-preview img').src.split(',').slice(1).join(','));
      const holder = document.createElement('div');
      holder.style.position = 'absolute';
      holder.style.visibility = 'hidden';
      holder.innerHTML = svg;
      document.body.appendChild(holder);
      const colors = {
        background: getComputedStyle(holder.querySelector('.frame-border-inner')).fill,
        participant: getComputedStyle(holder.querySelector('.participant-box')).fill,
        label: getComputedStyle(holder.querySelector('.participant-label')).fill,
        line: getComputedStyle(holder.querySelector('.message-line')).stroke,
        arrowFill: getComputedStyle(holder.querySelector('.arrow-head path')).fill,
        arrowStroke: getComputedStyle(holder.querySelector('.arrow-head path')).stroke,
      };
      holder.remove();
      return colors;
    });
    assert.deepEqual(zenColors, {
      background: 'rgb(8, 42, 84)', participant: 'rgb(13, 53, 102)',
      label: 'rgb(232, 229, 223)', line: 'rgb(59, 143, 209)',
      arrowFill: 'rgb(59, 143, 209)', arrowStroke: 'rgb(59, 143, 209)',
    });
    await page.waitForFunction(() => localStorage.getItem('solomon.mermaid.svg.v2')?.includes('data-solomon-zenuml-theme'));

    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
    await page.waitForFunction(() => typeof window.show === 'function');
    await page.evaluate(() => window.show('```mermaid\nsequenceDiagram\nAlice->>Bob: Hello\n```', false));
    await page.waitForFunction(() => !!document.querySelector('.chat-mermaid-preview'));
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-preview').getAttribute('aria-busy')), 'false');
    assert.equal(await page.evaluate(() => getComputedStyle(document.querySelector('.chat-mermaid-preview img')).animationName), 'none');
    await page.evaluate(() => window.show('```mermaid\nflowchart LR\nCompleted-->Direct\n```', false));
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => getComputedStyle(document.querySelector('.chat-mermaid-preview')).transitionProperty), 'none');
    assert.equal(await page.evaluate(() => getComputedStyle(document.querySelector('.chat-mermaid-preview img')).animationName), 'none');
    await page.evaluate(() => window.show('```mermaid\ntreemap-beta\n"Progetto"\n  "Frontend"\n    "GUI": 40\n    "TUI": 25\n  "Backend"\n    "API": 20\n    "Storage": 15\n```', false));
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    const treemapColors = await page.evaluate(() => {
      const svg = decodeURIComponent(document.querySelector('.chat-mermaid-preview img').src.split(',').slice(1).join(','));
      const holder = document.createElement('div');
      holder.innerHTML = svg;
      document.body.appendChild(holder);
      const leaf = holder.querySelector('.treemapLeaf');
      const colors = { fill: getComputedStyle(leaf).fill, stroke: getComputedStyle(leaf).stroke, opacity: getComputedStyle(leaf).fillOpacity };
      holder.remove();
      return colors;
    });
    assert.ok(['rgb(8, 42, 84)', 'rgb(13, 53, 102)'].includes(treemapColors.fill));
    assert.equal(treemapColors.stroke, 'rgb(59, 143, 209)');
    assert.equal(treemapColors.opacity, '1');
    await page.evaluate(() => window.show('```mermaid\nradar-beta\n axis v["Velocità"], q["Qualità"], s["Semplicità"]\n curve a["Soluzione A"]{8,9,7}\n curve b["Soluzione B"]{5,6,8}\n max 10\n```', false));
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    const radarColors = await page.evaluate(() => {
      const svg = decodeURIComponent(document.querySelector('.chat-mermaid-preview img').src.split(',').slice(1).join(','));
      const holder = document.createElement('div');
      holder.innerHTML = svg;
      document.body.appendChild(holder);
      const colors = {
        first: getComputedStyle(holder.querySelector('.radarCurve-0')).stroke,
        second: getComputedStyle(holder.querySelector('.radarCurve-1')).stroke,
        legend: getComputedStyle(holder.querySelector('.radarLegendBox-1')).fill,
        grid: getComputedStyle(holder.querySelector('.radarGraticule')).fill,
      };
      holder.remove();
      return colors;
    });
    assert.deepEqual(radarColors, { first: 'rgb(59, 143, 209)', second: 'rgb(255, 199, 4)', legend: 'rgb(255, 199, 4)', grid: 'none' });
    await page.evaluate(() => window.show('```mermaid\nrequirementDiagram\n requirement accesso {\n id: R1\n text: Accesso autenticato\n risk: high\n verifymethod: test\n }\n element verifica {\n type: test\n }\n verifica - verifies -> accesso\n```', false));
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    const sizing = await page.evaluate(() => ({
      compact: window.diagramSize('<svg viewBox="0 0 300 200"/>', 600),
      fitted: window.diagramSize('<svg viewBox="0 0 400 600"/>', 600),
      tall: window.diagramSize('<svg viewBox="0 0 400 1800"/>', 600),
    }));
    assert.deepEqual(sizing.compact, { width: 300, height: 200, units: 1 });
    assert.deepEqual(sizing.fitted, { width: 320, height: 480, units: 1 });
    assert.deepEqual(sizing.tall, { width: 373, height: 1680, units: 3 });
    const radarCode = '```mermaid\nradar-beta\n axis A,B,C\n curve a{1,2,3}\n```';
    await page.evaluate(content => window.show(content, false, 450), radarCode);
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    const beforeResize = await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').src);
    await page.evaluate(content => window.show(content, false, 700), radarCode);
    await page.waitForFunction(() => Number(document.querySelector('.chat-mermaid-preview img').dataset.diagramUnits) === 1);
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').src), beforeResize);
    await page.evaluate(() => window.show('```mermaid\nsankey-beta\n\nEntrate,Sviluppo,60\nEntrate,Infrastruttura,25\nEntrate,Supporto,15\nSviluppo,GUI,35\nSviluppo,TUI,25\n```', false));
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    const sankeyColors = await page.evaluate(() => {
      const svg = decodeURIComponent(document.querySelector('.chat-mermaid-preview img').src.split(',').slice(1).join(','));
      const holder = document.createElement('div');
      holder.innerHTML = svg;
      document.body.appendChild(holder);
      const result = {
        blend: getComputedStyle(holder.querySelector('.link')).mixBlendMode,
        opacity: getComputedStyle(holder.querySelector('.links')).strokeOpacity,
        source: holder.querySelector('.nodes .node rect').getAttribute('fill'),
        gradients: holder.querySelectorAll('.links linearGradient').length,
        target: holder.querySelectorAll('.nodes .node rect')[1].getAttribute('fill'),
        stroke: holder.querySelector('.link path').getAttribute('stroke'),
      };
      holder.remove();
      return result;
    });
    assert.equal(sankeyColors.blend, 'normal');
    assert.equal(sankeyColors.opacity, '1');
    assert.equal(sankeyColors.source, '#3b8fd1');
    assert.equal(sankeyColors.gradients, 0);
    assert.equal(sankeyColors.stroke, sankeyColors.target);
    await page.evaluate(() => window.show('```mermaid\nkanban\n todo[Da fare]\n  task1[Scrivere test]\n  task2[Aggiornare documentazione]\n doing[In corso]\n  task3[Renderer TUI]\n done[Completato]\n  task4[Prototipo]\n```', false));
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview')?.getAttribute('aria-busy') === 'false');
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    const kanbanColors = await page.evaluate(() => {
      const svg = decodeURIComponent(document.querySelector('.chat-mermaid-preview img').src.split(',').slice(1).join(','));
      const holder = document.createElement('div');
      holder.innerHTML = svg;
      document.body.appendChild(holder);
      const section = getComputedStyle(holder.querySelector('.sections rect'));
      const card = getComputedStyle(holder.querySelector('.items rect'));
      const result = { background: section.fill, border: section.stroke, card: card.fill, cardBorder: card.stroke };
      holder.remove();
      return result;
    });
    assert.deepEqual(kanbanColors, { background: 'rgb(8, 42, 84)', border: 'rgb(23, 72, 117)', card: 'rgb(13, 53, 102)', cardBorder: 'rgb(59, 143, 209)' });
    const themedPaints = await page.evaluate(() => {
      const svg = window.themeDiagram('unknownDiagram', `<svg xmlns="http://www.w3.org/2000/svg">
        <style>.box {fill:rgb(0, 0, 0)} .label {fill:#222} .edge {stroke:black} .transparent {fill:none}</style>
        <rect class="box" width="20" height="20"/><text class="label">Label</text>
        <path class="edge" stroke="#000"/><path class="arrow" fill="black"/>
        <rect class="inline" style="fill:#111;stroke:rgb(0,0,0)"/>
        <circle class="colored" fill="#ffc704"/><rect class="transparent"/>
        <rect class="clear" fill="transparent"/>
      </svg>`);
      const holder = document.createElement('div');
      holder.innerHTML = svg;
      document.body.appendChild(holder);
      const value = (selector, property) => getComputedStyle(holder.querySelector(selector))[property];
      const result = { box: value('.box','fill'), label: value('.label','fill'), edge: value('.edge','stroke'), arrow: value('.arrow','fill'), inline: value('.inline','fill'), colored: value('.colored','fill'), none: value('.transparent','fill'), clear: value('.clear','fill'), stable: window.themeDiagram('unknownDiagram', svg) === svg };
      holder.remove();
      return result;
    });
    assert.deepEqual(themedPaints, { box:'rgb(8, 42, 84)', label:'rgb(232, 229, 223)', edge:'rgb(59, 143, 209)', arrow:'rgb(59, 143, 209)', inline:'rgb(8, 42, 84)', colored:'rgb(255, 199, 4)', none:'none', clear:'rgba(0, 0, 0, 0)', stable:true });
    const many = Array.from({ length: 30 }, () => '```mermaid\nflowchart LR\nShared-->Result\n```').join('\n\n');
    await page.evaluate(content => window.show(content, false), many);
    await page.waitForFunction(() => document.querySelectorAll('.chat-mermaid-preview img').length === 30 && !document.querySelector('.chat-mermaid-preview[aria-busy="true"]'));
    assert.equal(await page.evaluate(() => new Set([...document.querySelectorAll('.chat-mermaid-preview img')].map(image => image.src)).size), 1);
    await page.evaluate(() => window.show('Other chat', false));
    await page.waitForFunction(() => !document.querySelector('.chat-mermaid-preview'));
    await page.evaluate(content => window.show(content, false), many);
    await page.waitForFunction(() => document.querySelectorAll('.chat-mermaid-preview').length === 30);
    assert.equal(await page.evaluate(() => document.querySelectorAll('.chat-mermaid-preview[aria-busy="true"]').length), 0);




  } finally {
    await browser?.close();
    await server.close();
    await rm(cacheDir, { recursive: true, force: true });
  }
});
