package tooling

import (
	"encoding/json"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func formatCreatePlanToolDisplayLines(m map[string]json.RawMessage) []string {
	name := jsonDisplayString(m["name"])
	body := jsonDisplayString(m["goal"])
	if name != "" && body != "" {
		body = name + " • " + body
	} else if name != "" {
		body = name
	}
	return []string{termcolor.ToolHeaderLine("createPlan", body)}
}

func formatEditPlanToolDisplayLines(m map[string]json.RawMessage) []string {
	return formatTextDiffToolDisplayLines("editPlan", jsonDisplayString(m["name"]), jsonDisplayString(m["old"]), jsonDisplayString(m["new"]))
}

func formatBuildPlanToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{termcolor.ToolHeaderLine("buildPlan", jsonDisplayString(m["name"]))}
}

func formatAddTodoToolDisplayLines(m map[string]json.RawMessage) []string {
	name := jsonDisplayString(m["name"])
	todo := jsonDisplayString(m["todo"])
	body := todo
	if name != "" && todo != "" {
		body = name + " • " + todo
	} else if name != "" {
		body = name
	}
	return []string{termcolor.ToolHeaderLine("addTodo", body)}
}

func formatTodoListToolDisplayLines(m map[string]json.RawMessage) []string {
	name := jsonDisplayString(m["name"])
	if name == "" {
		return []string{termcolor.ToolHeaderLine("todoList", "active plan")}
	}
	return []string{termcolor.ToolHeaderLine("todoList", name)}
}

func formatCheckTodoToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{termcolor.ToolHeaderLine("checkTodo", jsonDisplayString(m["sha1"]))}
}

func formatRemoveTodoToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{termcolor.ToolHeaderRedArgLine("removeTodo", jsonDisplayString(m["sha1"]))}
}

func formatCheckPlanToolDisplayLines(m map[string]json.RawMessage) []string {
	name := jsonDisplayString(m["name"])
	if jsonDisplayBool(m["full"]) {
		if name != "" {
			name += " • full"
		} else {
			name = "full"
		}
	}
	return []string{termcolor.ToolHeaderLine("checkPlan", name)}
}

func formatDeletePlanToolDisplayLines(m map[string]json.RawMessage) []string {
	return []string{termcolor.ToolHeaderRedArgLine("deletePlan", jsonDisplayString(m["name"]))}
}

func formatTextDiffToolDisplayLines(toolName, label, oldS, newS string) []string {
	out := []string{termcolor.ToolHeaderLine(toolName, label)}
	removed, added := editFileDiffRemovedAdded(oldS, newS)
	out = append(out, formatEditFileDiffSide(removed, termcolor.WrapEditFileOldStringLine)...)
	out = append(out, formatEditFileDiffSide(added, termcolor.WrapEditFileNewStringLine)...)
	return out
}

func formatPlanToolResultBody(toolName string, m map[string]json.RawMessage) string {
	switch toolName {
	case "createPlan":
		if p := jsonDisplayString(m["path"]); p != "" {
			body := "→ " + p
			if n, ok := jsonDisplayInt(m["pending_plans"]); ok {
				body += fmt.Sprintf(" (%d pending)", n)
			}
			return body
		}
	case "buildPlan":
		if g := jsonDisplayString(m["goal"]); g != "" {
			return "→ " + firstDisplayLine(g, 120)
		}
	case "addTodo":
		if sha := jsonDisplayString(m["sha"]); sha != "" {
			return "→ " + sha
		}
	case "checkTodo", "removeTodo":
		if st := jsonDisplayString(m["status"]); st != "" {
			return "→ " + st
		}
	case "checkPlan":
		if st := jsonDisplayString(m["status"]); st != "" {
			body := "→ " + st
			if n, ok := jsonDisplayInt(m["remaining"]); ok {
				body += fmt.Sprintf(" (%d open)", n)
			}
			return body
		}
	case "deletePlan":
		if p := jsonDisplayString(m["path"]); p != "" {
			return "→ " + p
		}
	case "todoList":
		return ""
	}
	return ""
}
