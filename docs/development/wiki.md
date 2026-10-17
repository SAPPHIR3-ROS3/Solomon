# Documentation publishing: GitHub Wiki and Pages

The source of truth is `docs/`. The [Wiki workflow](../../.github/workflows/wiki.yml) validates pull requests and publishes changes on `main`. Generator, test and workflow changes also trigger it. Manual dispatch supports retries.

The [Pages workflow](../../.github/workflows/pages.yml) independently builds the website and a responsive, JavaScript-free Wiki from the same source. Neither workflow waits for or calls the other; Pages does not require `WIKI_TOKEN`.

## GitHub Wiki initial setup

1. Enable Wiki in repository settings and create its first page through GitHub. Enabling Wiki alone does not initialize its separate Git repository.
2. Add an Actions repository secret named `WIKI_TOKEN`. Its owner must have repository write access; a classic PAT needs `public_repo` for a public repository or `repo` for a private one. Authorize organization SSO if required.
3. Run **Actions → Wiki → Run workflow** on `main`.

The default Actions token is not used to push to the separate wiki repository. Never put tokens in source or clone URLs. PR validation does not use the publishing secret. Missing credentials or an uninitialized wiki produce explicit errors.

## GitHub Wiki mapping and ownership

Every file under `docs/` is mirrored, including assets and supporting files. Article content is preserved except for navigation links required by GitHub Wiki.

- `README.md` becomes `Home.md`.
- Other Markdown paths are joined with `--`: `user-guide/installation.md` becomes `user-guide--installation.md`. Portal indexes remain distinct.
- Non-Markdown files retain their relative paths and bytes.
- Relative article links become wiki links; anchors are retained. Source links outside `docs/` point to the published source commit. Remote links and fenced examples stay unchanged.

This is a one-to-one content mirror, not a byte-identical folder layout. Deletions and renames remove obsolete pages. **Wiki-only files are removed during synchronization**; edit `docs/` rather than the published mirror. Git metadata and history are preserved; publication never force-pushes.

## Pages rendering and CI

`scripts/build_site.py` invokes the local Go renderer in `scripts/site/`. It uses Goldmark with GitHub Flavored Markdown (GFM), an existing dependency pinned in `go.mod`; no new dependencies are needed.

- At every directory level, `README.md` becomes `index.html`; other `.md` files become `.html`, preserving directories under `/wiki/`.
- All non-Markdown assets and supporting files are copied with their relative paths and bytes unchanged.
- Relative article and asset links are rewritten for the generated layout, preserving queries and anchors. Links outside `docs/` point to GitHub source at the supplied commit revision. Remote links and fenced examples remain unchanged.
- Mermaid fences remain readable code only: no diagram JavaScript is loaded. Navigation and article layouts are responsive and work without JavaScript.

Pages validates pushes to `main` and pull requests touching `docs/**`, `site/**`, `scripts/build_site.py`, `scripts/site/**`, `test/site_test.py`, `go.mod`, `go.sum`, or its workflow; manual dispatch is also available. The build job has read-only repository permissions, uses checkout with `persist-credentials: false`, and needs no publishing secrets. Only the separate deploy job on `main` receives `pages: write` and `id-token: write`.

CI uses `actions/setup-go` with `go-version-file: go.mod` and explicitly runs `go mod download github.com/yuin/goldmark` before testing or building, so a cold module cache works. This provisions only the existing pinned renderer dependency. The Python builder forces `GOPROXY=off GOTOOLCHAIN=local GOSUMDB=off` and invokes Go with `-mod=readonly`, preventing autonomous module or toolchain downloads during rendering.

## Local checks

```bash
python3 -m unittest discover -s test -p wiki_test.py
python3 scripts/wiki.py --output /tmp/solomon-wiki-preview --repository SAPPHIR3-ROS3/Solomon --revision main
make check-docs
```

The output directory is disposable: existing files are removed. Never choose an unrelated directory. The generator rejects overlapping source/output directories and colliding page names before cleanup. The GitHub Wiki generator uses Python standard-library facilities; Pages additionally requires a compatible local Go toolchain and the pinned Goldmark dependency already cached. See [website checks and preview](site-performance.md) for exact offline commands. Pages regenerates its disposable `wiki/` output, removing stale pages and assets.

## See also

- [Contributing](contributing.md)
- [Documentation index](../README.md)
