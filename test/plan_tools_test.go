package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/plan"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

const testProjHex = "93619f1ceceeb7a95e04d2d628313536bbde0774ac260359b480be61e04b58d2"

func setupPlanTest(t *testing.T) (string, *chatstore.Session) {
	t.Helper()
	stopCursorSidecar(t)
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	plansDir, err := chatstore.PlansDir(testProjHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sess := &chatstore.Session{}
	return plansDir, sess
}

func planTestEnv(t *testing.T, sess *chatstore.Session) *tools.Env {
	return &tools.Env{
		ProjHex:  testProjHex,
		ProjRoot: t.TempDir(),
		PlanningActive: func() bool {
			return sess.PlanningActive
		},
		ActivePlanName: func() string {
			return sess.ActivePlanName
		},
		SetPlanningActive: func(name string) {
			sess.PlanningActive = name != ""
			sess.ActivePlanName = name
		},
		SetPlanImplementing: func(v bool) {
			sess.PlanImplementing = v
		},
	}
}

func TestParseSectionsEmptyDesign(t *testing.T) {
	body := plan.SkeletonBody("Verificare il planning")
	body = []byte(string(body) + plan.FormatTodoLine("Scrivere nota", false) + "\n")
	sec := plan.ParseSections(body)
	if sec.Design != "" {
		t.Fatalf("design should be empty, got %q", sec.Design)
	}
	if sec.Goal != "Verificare il planning" {
		t.Fatalf("goal %q", sec.Goal)
	}
	if len(sec.Todo.Checklist) != 1 {
		t.Fatalf("checklist %d", len(sec.Todo.Checklist))
	}
}

func TestBuildPlanEmptyDesignExcerpt(t *testing.T) {
	plansDir, sess := setupPlanTest(t)
	sess.PlanningActive = true
	env := planTestEnv(t, sess)
	p, _ := plan.ResolvePath(plansDir, "brief.md")
	todoLine := plan.FormatTodoLine("Do work", false)
	meta := plan.NewMeta(plan.GitMeta{}, plan.StatusNotBuilt)
	body := "# Goal\n\nG\n\n## Context\n\n## Design\n\n## Todo\n\n" + todoLine + "\n"
	doc, _ := plan.WriteDocument(meta, []byte(body))
	_ = plan.WriteFile(p, doc)
	raw, _ := json.Marshal(map[string]string{"name": "brief.md"})
	out, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "buildPlan", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["ok"] != true {
		t.Fatalf("buildPlan: %v", m)
	}
	if excerpt, _ := m["design_excerpt"].(string); excerpt != "" {
		t.Fatalf("design_excerpt should be empty, got %q", excerpt)
	}
}

func TestPlanPackageTodoSHAAndStatus(t *testing.T) {
	text := "Setup package"
	sha := plan.TodoSHA(text)
	if len(sha) != 40 {
		t.Fatalf("sha len %d", len(sha))
	}
	line := plan.FormatTodoLine(text, false)
	if !strings.Contains(line, sha) {
		t.Fatalf("line missing sha: %s", line)
	}
	items := plan.ParseChecklist([]string{line, plan.FormatTodoLine("done", true)})
	if plan.StatusFromItems(items) != plan.StatusPartiallyBuilt {
		t.Fatalf("status %s", plan.StatusFromItems(items))
	}
}

func TestCreatePlanSkeleton(t *testing.T) {
	_, sess := setupPlanTest(t)
	sess.PlanningActive = true
	env := planTestEnv(t, sess)

	raw, _ := json.Marshal(map[string]string{"name": "feature.md", "goal": "Add planning"})
	out, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "createPlan", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["ok"] != true {
		t.Fatalf("createPlan: %v", m)
	}
	p := m["path"].(string)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	meta, body, err := plan.ParseDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Status != plan.StatusNotBuilt {
		t.Fatalf("status %s", meta.Status)
	}
	if !strings.Contains(string(body), "# Goal") || !strings.Contains(string(body), "## Todo") {
		t.Fatalf("body %s", body)
	}
	if !sess.PlanningActive || sess.ActivePlanName != "feature.md" {
		t.Fatalf("session not activated: %+v", sess)
	}
}

func TestAddTodoCheckTodoAppendEnd(t *testing.T) {
	plansDir, sess := setupPlanTest(t)
	env := planTestEnv(t, sess)
	env.SetPlanningActive("feature.md")

	createRaw, _ := json.Marshal(map[string]string{"name": "feature.md", "goal": "G"})
	if _, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "createPlan", Args: createRaw}); err != nil {
		t.Fatal(err)
	}

	bodyBefore := `## Todo

` + "```mermaid\nflowchart TD\n  A --> B\n```\n"
	p, _ := plan.ResolvePath(plansDir, "feature.md")
	meta := plan.NewMeta(plan.GitMeta{}, plan.StatusNotBuilt)
	doc, _ := plan.WriteDocument(meta, []byte("# Goal\n\nG\n\n## Context\n\n## Design\n\n"+bodyBefore))
	_ = plan.WriteFile(p, doc)

	addRaw, _ := json.Marshal(map[string]string{"name": "feature.md", "todo": "First task"})
	out, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "addTodo", Args: addRaw})
	if err != nil {
		t.Fatal(err)
	}
	sha := out.(map[string]any)["sha"].(string)

	b, _ := os.ReadFile(p)
	if !strings.HasSuffix(strings.TrimSpace(string(b)), sha) {
		t.Fatalf("todo not at end: %s", b)
	}

	chkRaw, _ := json.Marshal(map[string]string{"sha1": sha})
	out2, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "checkTodo", Args: chkRaw})
	if err != nil {
		t.Fatal(err)
	}
	if out2.(map[string]any)["status"] != plan.StatusBuilt {
		t.Fatalf("expected built got %v", out2)
	}
}

func TestBuildPlanBriefNoNested(t *testing.T) {
	plansDir, sess := setupPlanTest(t)
	sess.PlanningActive = true
	env := planTestEnv(t, sess)
	p, _ := plan.ResolvePath(plansDir, "impl.md")
	todoLine := plan.FormatTodoLine("Do work", false)
	sec := plan.ParseChecklist([]string{todoLine})
	meta := plan.NewMeta(plan.GitMeta{}, plan.StatusFromItems(sec))
	body := "# Goal\n\nMy goal\n\n## Context\n\nctx\n\n## Design\n\ndesign\n\n## Todo\n\nrules\n\n```mermaid\nflowchart TD\n  A --> B\n```\n\n" + todoLine + "\n"
	doc, _ := plan.WriteDocument(meta, []byte(body))
	_ = plan.WriteFile(p, doc)

	raw, _ := json.Marshal(map[string]string{"name": "impl.md"})
	out, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "buildPlan", Args: raw})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["ok"] != true || m["goal"] != "My goal" {
		t.Fatalf("brief %v", m)
	}
	if _, ok := m["summary"]; ok {
		t.Fatal("buildPlan must not nested-run")
	}
	if !sess.PlanImplementing {
		t.Fatal("PlanImplementing not set")
	}
}

func TestSlashPlanActivatesPlanning(t *testing.T) {
	sess := &chatstore.Session{}
	d := testDeps(sess)
	var mode string
	d.SetMode = func(m string) { mode = m }
	if err := agent.SlashDispatch(d, "/plan"); err != nil {
		t.Fatal(err)
	}
	if mode != "agent" || !sess.PlanningActive {
		t.Fatalf("mode=%s planning=%v", mode, sess.PlanningActive)
	}
}

func TestPlanNativeToolsWhenPlanningActive(t *testing.T) {
	params, err := tools.NativeToolParams("agent")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"createPlan", "addTodo"} {
		found := false
		for _, p := range params {
			if p.GetFunction().Name == name {
				found = true
			}
		}
		if found {
			t.Fatalf("%s should not be in default agent params", name)
		}
	}
	planParams := tools.PlanNativeToolParams()
	if len(planParams) < 9 {
		t.Fatalf("plan params %d", len(planParams))
	}
}

func TestCheckPlanFullAndDelete(t *testing.T) {
	plansDir, sess := setupPlanTest(t)
	sess.PlanningActive = true
	env := planTestEnv(t, sess)
	p, _ := plan.ResolvePath(plansDir, "x.md")
	meta := plan.NewMeta(plan.GitMeta{}, plan.StatusNotBuilt)
	doc, _ := plan.WriteDocument(meta, plan.SkeletonBody("goal"))
	_ = plan.WriteFile(p, doc)

	full := true
	chkRaw, _ := json.Marshal(map[string]any{"name": "x.md", "full": full})
	out, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "checkPlan", Args: chkRaw})
	if err != nil {
		t.Fatal(err)
	}
	if out.(map[string]any)["body"] == nil {
		t.Fatal("expected body")
	}

	delRaw, _ := json.Marshal(map[string]string{"name": "x.md"})
	if _, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "deletePlan", Args: delRaw}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
}

func TestCountPendingPlans(t *testing.T) {
	plansDir, _ := setupPlanTest(t)
	for _, name := range []string{"a.md", "b.md"} {
		p := filepath.Join(plansDir, name)
		meta := plan.NewMeta(plan.GitMeta{}, plan.StatusNotBuilt)
		doc, _ := plan.WriteDocument(meta, plan.SkeletonBody("g"))
		_ = plan.WriteFile(p, doc)
	}
	n, err := plan.CountPending(plansDir)
	if err != nil || n != 2 {
		t.Fatalf("pending=%d err=%v", n, err)
	}
}
