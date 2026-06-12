package tools

import (
	"encoding/json"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/plan"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureAddTodo(name, todo string) {}
func signatureTodoList() {}
func signatureCheckTodo(sha1 string) {}
func signatureRemoveTodo(sha1 string) {}

type addTodoArgs struct {
	Name string `json:"name"`
	Todo string `json:"todo"`
}

type planNameArgs struct {
	Name string `json:"name"`
}

type shaArgs struct {
	SHA1 string `json:"sha1"`
}

func addTodoOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("addTodo", "Append an open todo as the last line of the plan file.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Plan filename"},
		"todo": map[string]any{"type": "string", "description": "One-sentence todo"},
	}, []string{"name", "todo"})
}

func todoListOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("todoList", "List open todos for the active or named plan.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Optional plan filename; defaults to active plan"},
	}, []string{})
}

func checkTodoOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("checkTodo", "Mark a todo done by SHA1.", map[string]any{
		"sha1": map[string]any{"type": "string", "description": "SHA1 hex digest of the todo"},
	}, []string{"sha1"})
}

func removeTodoOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("removeTodo", "Remove a todo line by SHA1.", map[string]any{
		"sha1": map[string]any{"type": "string", "description": "SHA1 hex digest of the todo"},
	}, []string{"sha1"})
}

func appendAddTodoDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureAddTodo)
	if err != nil {
		return err
	}
	b.addBlock("addTodo", "Append an open todo as the last line of the plan file.", sig)
	return nil
}

func appendTodoListDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureTodoList)
	if err != nil {
		return err
	}
	b.addBlock("todoList", "List open todos for the active or named plan.", sig)
	return nil
}

func appendCheckTodoDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureCheckTodo)
	if err != nil {
		return err
	}
	b.addBlock("checkTodo", "Mark a todo done by SHA1.", sig)
	return nil
}

func appendRemoveTodoDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureRemoveTodo)
	if err != nil {
		return err
	}
	b.addBlock("removeTodo", "Remove a todo line by SHA1.", sig)
	return nil
}

func execAddTodo(env *Env, raw json.RawMessage) (any, error) {
	var a addTodoArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.Todo == "" {
		return nil, fmt.Errorf("todo is required")
	}
	p, err := planPath(env, a.Name)
	if err != nil {
		return nil, err
	}
	b, err := readPlanBytes(p)
	if err != nil {
		return nil, err
	}
	meta, body, err := plan.ParseDocument(b)
	if err != nil {
		return nil, err
	}
	newBody := plan.AppendTodoLine(string(body), a.Todo)
	sec := plan.ParseSections([]byte(newBody))
	meta.Status = plan.StatusFromItems(sec.Todo.Checklist)
	doc, err := plan.WriteDocument(meta, []byte(newBody))
	if err != nil {
		return nil, err
	}
	if err := writePlanBytes(p, doc); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "sha": plan.TodoSHA(a.Todo), "status": meta.Status}, nil
}

func execTodoList(env *Env, raw json.RawMessage) (any, error) {
	var a planNameArgs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &a)
	}
	p, err := planPath(env, a.Name)
	if err != nil {
		return nil, err
	}
	_, sec, _, err := plan.ReadFile(p)
	if err != nil {
		return nil, err
	}
	open := plan.OpenTodos(sec.Todo.Checklist)
	out := make(map[string]string, len(open))
	for _, it := range open {
		out[it.SHA] = it.Text
	}
	return out, nil
}

func execCheckTodo(env *Env, raw json.RawMessage) (any, error) {
	var a shaArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.SHA1 == "" {
		return nil, fmt.Errorf("sha1 is required")
	}
	p, err := findPlanByTodoSHA(env, a.SHA1)
	if err != nil {
		return map[string]any{"ok": false, "reason": "sha not found"}, nil
	}
	b, err := readPlanBytes(p)
	if err != nil {
		return nil, err
	}
	meta, body, err := plan.ParseDocument(b)
	if err != nil {
		return nil, err
	}
	newBody, ok, err := plan.ReplaceTodoChecked(string(body), a.SHA1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return map[string]any{"ok": false, "reason": "sha not found"}, nil
	}
	sec := plan.ParseSections([]byte(newBody))
	meta.Status = plan.StatusFromItems(sec.Todo.Checklist)
	doc, err := plan.WriteDocument(meta, []byte(newBody))
	if err != nil {
		return nil, err
	}
	if err := writePlanBytes(p, doc); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "status": meta.Status}, nil
}

func execRemoveTodo(env *Env, raw json.RawMessage) (any, error) {
	var a shaArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.SHA1 == "" {
		return nil, fmt.Errorf("sha1 is required")
	}
	p, err := findPlanByTodoSHA(env, a.SHA1)
	if err != nil {
		return map[string]any{"ok": false, "reason": "sha not found"}, nil
	}
	b, err := readPlanBytes(p)
	if err != nil {
		return nil, err
	}
	meta, body, err := plan.ParseDocument(b)
	if err != nil {
		return nil, err
	}
	newBody, ok, err := plan.RemoveTodoLine(string(body), a.SHA1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return map[string]any{"ok": false, "reason": "sha not found"}, nil
	}
	sec := plan.ParseSections([]byte(newBody))
	meta.Status = plan.StatusFromItems(sec.Todo.Checklist)
	doc, err := plan.WriteDocument(meta, []byte(newBody))
	if err != nil {
		return nil, err
	}
	if err := writePlanBytes(p, doc); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "status": meta.Status}, nil
}
