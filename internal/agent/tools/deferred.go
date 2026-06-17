package tools

func AgentDeferredToolNames() []string {
	return []string{
		"deepResearch", "researchStatus",
		"createPlan", "editPlan", "addTodo", "todoList", "checkTodo", "removeTodo", "checkPlan", "deletePlan",
		"shell", "readFile", "editFile", "find", "listDir", "tree", "fetchWeb", "webSearch",
	}
}

func isAgentDeferredTool(name string) bool {
	for _, n := range AgentDeferredToolNames() {
		if n == name {
			return true
		}
	}
	return false
}
