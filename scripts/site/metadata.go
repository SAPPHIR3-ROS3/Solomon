package main

// Keep search and sharing summaries close to the renderer, without adding
// publishing metadata to the Markdown mirrored to GitHub Wiki.
type articleMetadata struct {
	Title, Description, Language string
}

var wikiMetadata = map[string]articleMetadata{
	"README.md":                             {Title: "AI agent documentation and guides", Description: "Learn to install and use Solomon, configure cloud or local models, automate tasks, and explore the AI agent's architecture and development guides."},
	"features.md":                           {Title: "AI agent features and capabilities", Description: "Explore Solomon's terminal agent features, model integrations, tools, skills, MCP support and research workflows, with links to implementation guides."},
	"user-guide/README.md":                  {Title: "User guide: install, configure and use Solomon", Description: "Start using Solomon with guides to installation, model providers, commands, project instructions, terminal setup, data storage and troubleshooting."},
	"user-guide/installation.md":            {Title: "Install Solomon on Linux, macOS and Windows", Description: "Install the Solomon AI agent, check prerequisites, set up your shell PATH and learn where the CLI binary is installed on your operating system."},
	"user-guide/configuration.md":           {Title: "Configure model providers, MCP and agent settings", Description: "Configure Solomon's cloud and local model providers, MCP servers, web search and agent settings using config.toml and interactive setup commands."},
	"user-guide/usage-and-commands.md":      {Title: "AI agent quickstart, CLI and slash commands", Description: "Start your first Solomon session, use slash commands, run deep research and automate tasks with machine-readable JSON or JSONL command output."},
	"user-guide/data-layout.md":             {Title: "Local data storage, sessions and configuration", Description: "Find Solomon's configuration, chats, plans, skills and project data under ~/.solomon, and understand how workspaces organize locally stored state."},
	"user-guide/project-instructions.md":    {Title: "Project instructions, AGENTS.md and custom rules", Description: "Customize Solomon's behavior with AGENTS.md instruction files and numbered rules. Learn their scopes and how they enter the agent's system prompt."},
	"user-guide/terminal-setup.md":          {Title: "Terminal fonts, colors and display setup", Description: "Set up monospace fonts, disable ligatures and adjust terminal colors for readable Solomon prompts, aligned output and a correctly displayed banner."},
	"user-guide/troubleshooting.md":         {Title: "Troubleshoot installation, login, MCP and updates", Description: "Diagnose Solomon startup, desktop, authentication, MCP and update problems with focused checks that preserve your configuration and session data."},
	"user-guide/interface-and-terminal.md":  {Title: "Interfaccia grafica e terminale: guida alle funzioni", Description: "Scopri dove trovare le funzioni di Solomon nella GUI web, nell'app desktop e nel terminale, distinguendo le operazioni disponibili da quelle previste.", Language: "it"},
	"architecture/README.md":                {Title: "AI agent architecture and internal design", Description: "Explore Solomon's Go architecture, from CLI entry points and the runtime loop to model providers, tools, persistence and extension interfaces."},
	"architecture/overview.md":              {Title: "Local-first AI agent architecture overview", Description: "Understand Solomon's local-first design, Go binary, model provider interfaces and workspace state, with an overview of core package dependencies."},
	"architecture/agent-turn-pipeline.md":   {Description: "Follow a Solomon agent turn from system prompt construction and model streaming to message persistence, tool execution and context compaction."},
	"architecture/checkpoints.md":           {Description: "Understand Solomon session checkpoints, message versions, branching and truncation flows, including optional synchronization with Git commit IDs."},
	"architecture/cursor-integration.md":    {Description: "Explore Solomon's Cursor API sidecar, its OpenAI-compatible HTTP interface and workspace tool execution, and distinguish it from Cursor Sub."},
	"architecture/llm-layer.md":             {Description: "Learn how Solomon adapts messages to model provider APIs, streams responses and reasoning, tracks usage and checks the integrity of model streams."},
	"architecture/mcp-integration.md":       {Description: "Understand Solomon's MCP host and client architecture, server transports, deferred tool invocation, resources, prompts and authentication support."},
	"architecture/native-tools.md":          {Description: "Explore Solomon's built-in Go tools, tool-call parsing and execution, and the routing that connects native capabilities with skills and MCP."},
	"architecture/package-index.md":         {Description: "Find Solomon's Go packages and integration roots, grouped by responsibility, to locate CLI, runtime, model, storage and supporting implementations."},
	"architecture/plan-vs-build.md":         {Description: "Compare Solomon's agent and chat session modes, including the native tools and system prompt templates selected by each runtime mode."},
	"architecture/research.md":              {Description: "Trace Solomon's deep research workflow through web searches, page extraction, evidence synthesis and saved HTML reports scoped to a project."},
	"architecture/runtime-and-repl.md":      {Description: "Navigate Solomon's runtime documentation for the REPL editor, orchestration loop, tool execution, MCP, nested subagents and CI operation."},
	"architecture/runtime-orchestration.md": {Description: "Follow Solomon runtime orchestration across model turns, tool dispatch, persistence, MCP, checkpoints, nested subagents and machine-output mode."},
	"architecture/runtime-repl.md":          {Description: "Explore Solomon's interactive terminal input, editor, completions, file mentions and shell-first behavior, with links to orchestration callbacks."},
	"architecture/runtime.md":               {Description: "Locate Solomon's runtime core and subpackages for the REPL, model turns, tool dispatch, MCP, checkpoints and integrations, with debugging pointers."},
	"architecture/server.md":                {Description: "Understand the Solomon local server, its detached user-scoped lifecycle, web interface and localhost API for desktop projects, chats and streaming."},
	"architecture/sessions-and-storage.md":  {Description: "Learn how Solomon stores project chat transcripts, resumes sessions, resolves local paths and tracks images, subchats and background subagent state."},
	"architecture/skills-and-slash.md":      {Description: "Explore Solomon's slash command registry and skill scopes, including session control, configuration and CLI or REPL skill management."},
	"architecture/startup-and-cli.md":       {Description: "Follow Solomon binary startup, subcommand dispatch and interactive runtime construction, with the package and file map for the CLI entry flow."},
	"architecture/supporting-packages.md":   {Description: "Find the Solomon internal packages supporting terminal input, tools, authentication and interface behavior outside the core agent turn loop."},
	"development/README.md":                 {Title: "Developer guide: build, test and release Solomon", Description: "Build Solomon from source, run checks, contribute changes and publish releases, with guides to testing, performance and documentation deployment."},
	"development/building-and-releases.md":  {Description: "Build and test Solomon from source, run development checks and understand the release workflow for CLI and desktop distributions."},
	"development/chat-performance.md":       {Description: "Measure Solomon thread navigation, inspect timing phases and export chat performance reports without including message text or attachment data."},
	"development/contributing.md":           {Description: "Set up Solomon for development, make a focused contribution and follow the prerequisites, regression testing and documentation checklist."},
	"development/cookbook.md":               {Description: "Follow development recipes for changing Solomon, with the relevant files, implementation steps, tests and documentation for common contributions."},
	"development/gui-side-panels.md":        {Description: "Understand Solomon's desktop project sidebar and file explorer, including independent controls, resizing and persistence of preferred panel widths."},
	"development/installation-audit.md":     {Description: "Review Solomon installation reliability findings for source builds, release installers, coordinated updates, desktop provisioning and daemon discovery."},
	"development/memory-usage.md":           {Description: "Review measured Solomon development memory usage, test conditions and cold or warm Vite runs, including isolated browser JavaScript heap measurements."},
	"development/recent-improvements.md":    {Description: "Review recent Solomon changes across desktop chat, images, server runtime, model management, integrations and developer tooling."},
	"development/site-performance.md":       {Description: "Build and audit the Solomon website, understand inline CSS, local fonts and animations, and reproduce offline checks and Lighthouse measurements."},
	"development/site-seo.md":               {Description: "Maintain Solomon website metadata, canonical URLs and the generated wiki sitemap, and verify discovery and indexing through Google Search Console."},
	"development/testing.md":                {Description: "Understand Solomon's test layout, conventions and shared helpers, and choose the checks needed to verify changes to the CLI, runtime and GUI."},
	"development/wiki.md":                   {Description: "Publish Solomon documentation from one Markdown source to GitHub Wiki and GitHub Pages, with URL mappings, workflow setup and local validation."},
}
