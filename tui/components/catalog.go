package components

type Component struct {
	ID          string
	Label       string
	Description string
	States      []string
}

func Default() []Component {
	definitions := [][3]string{
		{"welcome", "Welcome", "Empty-thread banner and composer"},
		{"composer", "Composer", "Prompt, toolbar and current draft"},
		{"slash", "Slash commands", "Filtered command suggestions"},
		{"search_tools", "searchTools", "Readable tool discovery results"},
		{"orchestrate", "orchestrate", "Go source and execution output"},
		{"tool_result", "Tool result", "Complete tool output"},
		{"user", "User message", "User message frame"},
		{"image", "Image", "Decoded image pixels"},
		{"markdown", "Markdown", "Formatted prose, lists and tables"},
		{"subagent", "Subagent", "Nested agent status"},
		{"checkpoint", "Checkpoint", "Session checkpoint summary"},
		{"notification", "Notification", "Transient session notification"},
		{"transcript", "Transcript", "Long conversation viewport"},
		{"tool_call", "Tool call", "Complete intent, Go source, execution and result card"},
		{"file_summary", "File summary", "Changed files tree and line counts"},
		{"compaction", "Compaction", "Context summary and retained messages"},
		{"reasoning", "Reasoning", "Collapsible reasoning content"},
		{"worked_for", "Worked for", "Response elapsed time"},
		{"subagent_chat", "Subagent chat", "Nested conversation panel"},
		{"subagent_activity", "Subagent activity", "Active subagents indicator"},
		{"rate_limit", "Rate limit", "Provider rate limit notice"},
		{"interrupted", "Interrupted generation", "Interrupted response marker"},
		{"mode_switch", "Mode switch", "Mode transition notice"},
		{"edit_diff", "Edit diff", "File edit comparison"},
		{"message_actions", "Message actions", "Response action buttons"},
	}
	catalog := make([]Component, 0, len(definitions))
	for _, definition := range definitions {
		catalog = append(catalog, Component{ID: definition[0], Label: definition[1], Description: definition[2], States: []string{"expanded", "collapsed", "active"}})
	}
	for i := range catalog {
		if catalog[i].ID == "tool_call" {
			catalog[i].States = []string{"expanded", "collapsed", "active", "failed"}
		}
		if catalog[i].ID == "rate_limit" || catalog[i].ID == "interrupted" {
			catalog[i].States = []string{"visible"}
		}
	}
	return catalog
}
