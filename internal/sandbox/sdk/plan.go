package sdk

import "encoding/json"

func CreatePlan(name, goal string) (map[string]any, error) {
	raw, err := callTool("createPlan", map[string]any{"name": name, "goal": goal})
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

func BuildPlan(name string) (map[string]any, error) {
	raw, err := callTool("buildPlan", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func AddTodo(name, todo string) (map[string]any, error) {
	raw, err := callTool("addTodo", map[string]any{"name": name, "todo": todo})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func TodoList(name string) (map[string]string, error) {
	args := map[string]any{}
	if name != "" {
		args["name"] = name
	}
	raw, err := callTool("todoList", args)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func CheckTodo(sha1 string) (map[string]any, error) {
	raw, err := callTool("checkTodo", map[string]any{"sha1": sha1})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func RemoveTodo(sha1 string) (map[string]any, error) {
	raw, err := callTool("removeTodo", map[string]any{"sha1": sha1})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func CheckPlan(name string, full bool) (map[string]any, error) {
	raw, err := callTool("checkPlan", map[string]any{"name": name, "full": full})
	if err != nil {
		return nil, err
	}
	return decodeMap(raw)
}

func DeletePlan(name string) (map[string]any, error) {
	raw, err := callTool("deletePlan", map[string]any{"name": name})
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
