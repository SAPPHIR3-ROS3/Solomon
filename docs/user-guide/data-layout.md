# Data layout

Solomon stores user data outside the repository under `~/.solomon`. Project-scoped data is keyed by the canonical working directory and grouped under a stable project id ([`project.Resolve`](../../internal/project/project.go)).

```mermaid
flowchart LR
  home["~/.solomon/<br/><small>Solomon user data root</small>"]

  config["config.toml<br/><small>providers, model, user name,<br/>reasoning, language, logging,<br/>token caps, compaction</small>"]
  mcpConfig["mcp.json<br/><small>optional MCP servers</small>"]
  projectMap["projectsId.json<br/><small>canonical root -> 64-char id</small>"]
  logs["logs/<br/><small>file logs, 7-day retention</small>"]
  globalSkillsDir["skills/<br/><small>global skill files</small>"]
  skillsRegistry["skills.json<br/><small>global + per-project registry</small>"]

  projects["projects/<br/><small>per-project partitions</small>"]
  projectNode["&lt;project-id&gt;/"]
  chats["chats/<br/><small>session JSON</small>"]
  chatFile["*.json"]
  subchats["subchats/"]
  subchatFile["*.json"]
  plans["plans/<br/><small>plan markdown</small>"]
  planFile["*.md"]
  projectSkills["skills/"]

  workspaceRoot["&lt;workspace&gt;/.solomon/"]
  workspaceSkills["skills/"]
  localMirror["skills.json"]
  localFiles["..."]

  home --> config
  home --> mcpConfig
  home --> projectMap
  home --> logs
  home --> globalSkillsDir
  home --> skillsRegistry
  home --> projects
  projects --> projectNode
  projectNode --> chats
  chats --> chatFile
  chats --> subchats
  subchats --> subchatFile
  projectNode --> plans
  plans --> planFile
  projectNode --> projectSkills

  workspaceRoot --> workspaceSkills
  workspaceSkills --> localMirror
  workspaceSkills --> localFiles

  classDef folder fill:#eef6ff,stroke:#5b8def,color:#102a43
  classDef file fill:#fff7e6,stroke:#d9822b,color:#3d2b1f
  class home,logs,globalSkillsDir,projects,projectNode,chats,subchats,plans,projectSkills,workspaceRoot,workspaceSkills folder
  class config,mcpConfig,projectMap,skillsRegistry,chatFile,subchatFile,planFile,localMirror,localFiles file
```

## Session files

Chat sessions live under `projects/<project-id>/chats/*.json`. Each file holds session id, title, timestamps, messages, tool calls, checkpoint fields, token usage, and image references. See [Sessions and storage](../architecture/sessions-and-storage.md).

## Plans

Plan documents created through plan-mode tools are stored under `projects/<project-id>/plans/*.md`.

## Skills

- Global: `~/.solomon/skills/` + `skills.json`
- Per project: `projects/<project-id>/skills/`
- Per workspace: `<workspace>/.solomon/skills/` with local `skills.json` mirror

Registry and install paths: [Skills and slash](../architecture/skills-and-slash.md).

## See also

- [Configuration](configuration.md)
- [Sessions and storage](../architecture/sessions-and-storage.md)
- [Checkpoints](../architecture/checkpoints.md)
