# Website performance

The Pages workflow runs `scripts/build_site.py` before uploading the website.
The build includes each page's styles in its HTML, rebasing CSS font URLs for
the home page and the nested quickstart. This avoids render-blocking CSS requests
without requiring npm dependencies for deployment. The development files remain
separate in `site/styles/`.

To preview the production output:

```bash
python3 scripts/build_site.py --output dist/site
python3 -m http.server 8765 --directory dist/site
```

GitHub Pages compresses text assets. An uncompressed development server can
report slower simulated loading than the deployed website. Always check the
public URLs after deployment, with the same Lighthouse version and settings as
the baseline.

## Fonts and animation

Latin WOFF2 fonts are served from `site/assets/fonts/`; each font's SIL Open Font
License is included there. They were downloaded from the Google Fonts files used
by the original website. `font-display: optional` avoids late font swaps and
keeps content visible on slow connections. Only the small, critical heading font
is preloaded; the other fonts use the browser's normal loading priority.

The landing page serves a small anime.js 4.1.4 bundle locally. It includes only
the animation exports used by `motion.js`, with its MIT license under
`site/assets/vendor/`. To regenerate it, install the pinned build tools in a
temporary directory and use the checked-in entry point:

```bash
site_tool_dir=$(mktemp -d)
npm install --prefix "$site_tool_dir" --no-audit --no-fund animejs@4.1.4 esbuild@0.25.12
NODE_PATH="$site_tool_dir/node_modules" "$site_tool_dir/node_modules/.bin/esbuild" \
  scripts/site-anime-entry.mjs --bundle --minify --format=iife --target=es2020 \
  --outfile=site/assets/vendor/anime.iife.min.js
```

Main content paints without waiting for animations. The rotating headline
reserves space to avoid layout shifts. Phones use a static dot grid instead of
the interactive canvas. Reduced-motion users receive the static presentation.
Responsive overrides are loaded last so desktop section styles do not override
mobile layouts. The install tabs support arrow keys, Home and End, and connect
their selected label to the command panel. A skip link and visible focus outlines
support keyboard navigation.

## Lighthouse verification

Use an isolated headless Chrome profile; do not close or restart the user's
browser. The initial audits used Lighthouse 12.8.2 with its default simulated
mobile throttling. Baseline public scores were:

| Page | Performance | Accessibility | Best practices | SEO |
| --- | ---: | ---: | ---: | ---: |
| Home | 84 | 100 | 100 | 100 |
| Quickstart | 89 | 100 | 100 | 100 |

Install Lighthouse outside the repository and run the public audits:

```bash
site_audit_dir=$(mktemp -d)
npm install --prefix "$site_audit_dir" --no-audit --no-fund lighthouse@12.8.2
"$site_audit_dir/node_modules/.bin/lighthouse" \
  https://sapphir3-ros3.github.io/Solomon/ \
  --chrome-flags="--headless --disable-dev-shm-usage" \
  --only-categories=performance,accessibility,best-practices,seo \
  --output=json --output=html --output-path="$site_audit_dir/home-mobile"
```

Repeat for `/Solomon/getting-started/` and with `--preset=desktop`. Keep the HTML
and JSON reports for review. Scores can vary with network and machine conditions;
Lighthouse scores are lab measurements, not a guarantee of search ranking or a
replacement for real-user Core Web Vitals.
