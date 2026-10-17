# GitHub Wiki mirror

The source of truth is `docs/`. The [Wiki workflow](../../.github/workflows/wiki.yml) validates pull requests and publishes changes on `main`. Generator, test and workflow changes also trigger it. Manual dispatch supports retries.

## Initial setup

1. Enable Wiki in repository settings and create its first page through GitHub. Enabling Wiki alone does not initialize its separate Git repository.
2. Add an Actions repository secret named `WIKI_TOKEN`. Its owner must have repository write access; a classic PAT needs `public_repo` for a public repository or `repo` for a private one. Authorize organization SSO if required.
3. Run **Actions → Wiki → Run workflow** on `main`.

The default Actions token is not used to push to the separate wiki repository. Never put tokens in source or clone URLs. PR validation does not use the publishing secret. Missing credentials or an uninitialized wiki produce explicit errors.

## Mapping and ownership

Every file under `docs/` is mirrored, including assets and supporting files. Article content is preserved except for navigation links required by GitHub Wiki.

- `README.md` becomes `Home.md`.
- Other Markdown paths are joined with `--`: `user-guide/installation.md` becomes `user-guide--installation.md`. Portal indexes remain distinct.
- Non-Markdown files retain their relative paths and bytes.
- Relative article links become wiki links; anchors are retained. Source links outside `docs/` point to the published source commit. Remote links and fenced examples stay unchanged.

This is a one-to-one content mirror, not a byte-identical folder layout. Deletions and renames remove obsolete pages. **Wiki-only files are removed during synchronization**; edit `docs/` rather than the published mirror. Git metadata and history are preserved; publication never force-pushes.

## Local checks

```bash
python3 -m unittest discover -s test -p wiki_test.py
python3 scripts/wiki.py --output /tmp/solomon-wiki-preview --repository SAPPHIR3-ROS3/Solomon --revision main
make check-docs
```

The output directory is disposable: existing files are removed. Never choose an unrelated directory. The generator rejects overlapping source/output directories and colliding page names before cleanup. Only Python standard-library facilities and existing Go checks are required.

## See also

- [Contributing](contributing.md)
- [Documentation index](../README.md)
