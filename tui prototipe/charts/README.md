# Mermaid diagram fixtures

The TUI Mermaid gallery contains hand-arranged terminal diagrams and three raster previews. Pie, Sankey, and Radar PNG and SVG files are paired: SVG text positions are overlaid as native terminal text. The gallery is a prototype and does not render arbitrary Mermaid input.

Use Node.js and Chromium to regenerate the fixtures. Run `PUPPETEER_SKIP_DOWNLOAD=true npm install --no-package-lock` in this directory. Set `browser.json` to your Chromium path; the included `--no-sandbox` option is only suitable for trusted local inputs. Run `npm run render:themed -- -i pie.mmd -o ../fixtures/pie.png` and the same command for `pie.svg`, `radar.png`, and `radar.svg`. Generate the Sankey PNG and SVG with `npm run render` instead. Keep each PNG and SVG pair synchronized.
