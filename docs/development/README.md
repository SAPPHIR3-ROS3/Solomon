# Portal: Building & releasing

How to build, test, and tag Solomon from source.

## Articles

| Article | Summary |
|---------|---------|
| [building-and-releases.md](building-and-releases.md) | `go vet`, build, release workflow, CI |
| [testing.md](testing.md) | Test layout, pyramid, helpers, when to mock |
| [cookbook.md](cookbook.md) | Recipes: slash, tools, REPL, providers, Cursor |
| [ci-github-actions.example.yml](ci-github-actions.example.yml) | Example workflow: `--jsonl` stream and `--json` report |

## Suggested order

**Contributor**

1. [Building and releases — Development checks](building-and-releases.md#development-checks)
2. [Testing](testing.md)
3. [Cookbook](cookbook.md) — for the kind of change you are making
4. [Overview](../architecture/overview.md) — package map
5. Architecture article for the area (see [architecture portal](../architecture/README.md))

## See also

- [Main documentation index](../README.md)
- [Startup and CLI](../architecture/startup-and-cli.md)
