package sdk

import "encoding/json"

func CreatePlan(name, goal, intent string) (map[string]any, error) {
	raw, err := callTool("createPlan", map[string]any{"name": name, "goal": goal, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func EditPlan(name, old, new, intent string) (map[string]any, error) {
	raw, err := callTool("editPlan", map[string]any{"name": name, "old": old, "new": new, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func BuildPlan(name, intent string) (map[string]any, error) {
	raw, err := callTool("buildPlan", map[string]any{"name": name, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func AddTodo(name, todo, intent string) (map[string]any, error) {
	raw, err := callTool("addTodo", map[string]any{"name": name, "todo": todo, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func TodoList(name, intent string) (map[string]string, error) {
	args := map[string]any{"intent": intent}
	if name != "" {
		args["name"] = name
	}
	raw, err := callTool("todoList", args)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Todos map[string]string `json:"todos"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Todos != nil {
		return envelope.Todos, nil
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func CheckTodo(sha1, intent string) (map[string]any, error) {
	raw, err := callTool("checkTodo", map[string]any{"sha1": sha1, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func RemoveTodo(sha1, intent string) (map[string]any, error) {
	raw, err := callTool("removeTodo", map[string]any{"sha1": sha1, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func CheckPlan(name string, full bool, intent string) (map[string]any, error) {
	raw, err := callTool("checkPlan", map[string]any{"name": name, "full": full, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func DeletePlan(name, intent string) (map[string]any, error) {
	raw, err := callTool("deletePlan", map[string]any{"name": name, "intent": intent})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func decodeMap(raw json.RawMessage) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
