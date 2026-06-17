package tools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/plan"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
)

func planAllowed(env *Env, tool string) bool {
	if env != nil && env.AllowDeferredTools {
		return isPlanTool(tool)
	}
	if tool == "buildPlan" && env != nil && env.PlanningActive != nil && env.PlanningActive() {
		return true
	}
	return false
}

func PlanToolNames() []string {
	return []string{
		"createPlan", "editPlan", "buildPlan",
		"addTodo", "todoList", "checkTodo", "removeTodo",
		"checkPlan", "deletePlan",
	}
}

func isPlanTool(name string) bool {
	for _, t := range PlanToolNames() {
		if t == name {
			return true
		}
	}
	return false
}

func planPath(env *Env, name string) (string, error) {
	dir, err := chatPlansDir(env.ProjHex)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		if env.ActivePlanName != nil {
			name = env.ActivePlanName()
		}
	}
	return plan.ResolvePath(dir, name)
}

func resolvePlanName(env *Env, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		if env.ActivePlanName != nil {
			name = env.ActivePlanName()
		}
	}
	return project.NormalizePlanName(name)
}

func activatePlan(env *Env, name string) {
	if env.SetPlanningActive != nil {
		env.SetPlanningActive(name)
	}
}

func readPlanBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writePlanBytes(path string, b []byte) error {
	return plan.WriteFile(path, b)
}

func findPlanByTodoSHA(env *Env, sha string) (string, error) {
	name := ""
	if env.ActivePlanName != nil {
		name = env.ActivePlanName()
	}
	if name != "" {
		p, err := planPath(env, name)
		if err == nil {
			if _, sec, _, err := plan.ReadFile(p); err == nil {
				for _, it := range sec.Todo.Checklist {
					if it.SHA == sha {
						return p, nil
					}
				}
			}
		}
	}
	dir, err := chatPlansDir(env.ProjHex)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		_, sec, _, err := plan.ReadFile(p)
		if err != nil {
			continue
		}
		for _, it := range sec.Todo.Checklist {
			if it.SHA == sha {
				return p, nil
			}
		}
	}
	return "", os.ErrNotExist
}
