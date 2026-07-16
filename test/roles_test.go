package test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func withMockRolesModelLister(t *testing.T) {
	t.Helper()
	prev := config.RolesModelLister
	config.RolesModelLister = func(_ context.Context, _ *config.Root, p *config.Provider) ([]string, error) {
		if p == nil {
			return nil, context.Canceled
		}
		switch strings.TrimSpace(p.Name) {
		case "p":
			return []string{"m"}, nil
		case "openrouter":
			return []string{"qwen3-32b"}, nil
		case "groq":
			return []string{"llama-3.3-70b"}, nil
		default:
			return []string{"m", "qwen3-32b", "llama-3.3-70b", "other"}, nil
		}
	}
	t.Cleanup(func() { config.RolesModelLister = prev })
}

func testRolesConfig() *config.Root {
	return &config.Root{
		Roles: config.Roles{
			Table: config.RolesTable{
				Characteristics: []string{"reasoning", "cost", "speed"},
			},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "openrouter", Model: "qwen3-32b", Description: "explore"},
				{Provider: "groq", Model: "llama-3.3-70b", Description: "cheap", Scores: map[string]int{"cost": 90}},
			},
		},
	}
}

func testRolesEntries() []roles.SubagentEntry {
	return config.RolesSubagentEntries(testRolesConfig())
}

func TestRolesSubagentPool(t *testing.T) {
	pool := roles.SubagentPool(testRolesEntries())
	if len(pool) != 2 {
		t.Fatalf("pool: %+v", pool)
	}
}

func TestRolesFindSubagent(t *testing.T) {
	_, err := roles.FindSubagent(testRolesEntries(), "openrouter", "missing")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	e, err := roles.FindSubagent(testRolesEntries(), "openrouter", "qwen3-32b")
	if err != nil || e.Description != "explore" {
		t.Fatalf("entry=%+v err=%v", e, err)
	}
}

func TestListSubAgentsTool(t *testing.T) {
	env := &agenttools.Env{Cfg: testRolesConfig()}
	out, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "listSubAgents", Args: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("out=%#v", out)
	}
	if m["count"] != 2 {
		t.Fatalf("count=%v", m["count"])
	}
	table, _ := m["table"].(string)
	if !strings.Contains(table, "qwen3-32b") || !strings.Contains(table, "90") {
		t.Fatalf("manual table=%q", table)
	}
	if strings.Contains(table, "R") {
		t.Fatalf("benchmark/legacy score marker leaked into manual table=%q", table)
	}
}

func TestBuildManualTableViewIgnoresUnconfiguredScores(t *testing.T) {
	entries := []roles.SubagentEntry{
		{Provider: "p", Model: "m", Scores: map[string]int{"reasoning": 88, "cost": 77}},
	}
	view, err := roles.BuildManualTableView([]string{"reasoning", "speed"}, entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Rows[0].Scores["reasoning"]; got != 88 {
		t.Fatalf("manual reasoning=%d", got)
	}
	if _, ok := view.Rows[0].Scores["speed"]; ok {
		t.Fatal("unassigned speed score should remain empty")
	}
	if !view.Rows[0].Unclassified {
		t.Fatal("partial manual scores should be unclassified")
	}
}

func TestCompleteSubagentEntryCollectsManualScores(t *testing.T) {
	withMockRolesModelLister(t)
	cfg := &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning", "cost"}},
		},
	}
	answers := []string{"manual worker", "80", "not-a-score", "90"}
	answerIndex := 0
	var out bytes.Buffer
	pio := config.PromptIO{
		Out: &out,
		ReadLine: func(string) (string, error) {
			answer := answers[answerIndex]
			answerIndex++
			return answer, nil
		},
	}
	res, err := config.CompleteSubagentEntry(context.Background(), pio, cfg, "p", "m")
	if err != nil {
		t.Fatal(err)
	}
	if res.Description != "manual worker" || res.Scores["reasoning"] != 80 || res.Scores["cost"] != 90 {
		t.Fatalf("result=%+v", res)
	}
	if !strings.Contains(out.String(), "Enter an integer 0-100") {
		t.Fatalf("expected invalid-score retry, output=%q", out.String())
	}
	config.ApplySubagentAdd(cfg, res)
	if len(cfg.Roles.Subagent) != 1 || cfg.Roles.Subagent[0].Scores["cost"] != 90 {
		t.Fatalf("config=%+v", cfg.Roles.Subagent)
	}
}

func TestListSubAgentsWithoutTable(t *testing.T) {
	env := &agenttools.Env{Cfg: &config.Root{}}
	out, err := agenttools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "listSubAgents", Args: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["error"] == nil {
		t.Fatalf("expected error field: %#v", m)
	}
}

func TestConfigRolesValidationRejectsMissingProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	writeTestConfig(t, home, `
[providers.p]
base_url = "http://127.0.0.1:9"
api_key = "k"

[current]
provider = "p"
model = "m"

[roles.table]
characteristics = ["reasoning"]

[[roles.subagent]]
model = "qwen3-32b"
description = "broken"
`)
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected validation error for missing provider")
	}
}

func TestConfigRolesValidationAcceptsValidEntry(t *testing.T) {
	withMockRolesModelLister(t)
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	writeTestConfig(t, home, `
[providers.p]
base_url = "http://127.0.0.1:9"
api_key = "k"

[current]
provider = "p"
model = "m"

[roles.table]
characteristics = ["reasoning", "cost"]

[[roles.subagent]]
provider = "p"
model = "m"
description = "explore"

[roles.subagent.scores]
cost = 80
`)
	r, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Roles.Subagent) != 1 || r.Roles.Subagent[0].Model != "m" {
		t.Fatalf("roles: %+v", r.Roles.Subagent)
	}
	if err := config.Save(r); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRolesRejectsUnknownProvider(t *testing.T) {
	withMockRolesModelLister(t)
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Current:   config.Current{Provider: "p", Model: "m"},
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning"}},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "missing", Model: "m"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
}

func TestValidateRolesRejectsUnknownModel(t *testing.T) {
	withMockRolesModelLister(t)
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Current:   config.Current{Provider: "p", Model: "m"},
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning"}},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "unknown"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown model error")
	}
}

func TestValidateRolesAcceptsListedModel(t *testing.T) {
	withMockRolesModelLister(t)
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{
			"q": {Name: "q"},
		},
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning"}},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "q", Model: "other"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateRolesRejectsDuplicatePair(t *testing.T) {
	withMockRolesModelLister(t)
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Current:   config.Current{Provider: "p", Model: "m"},
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning"}},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m"},
				{Provider: "p", Model: "m"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate pair error")
	}
}

func TestValidateRolesRejectsSubagentWithoutTable(t *testing.T) {
	withMockRolesModelLister(t)
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Roles: config.Roles{
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected missing table error")
	}
}

func TestValidateRolesRejectsUnreachableProvider(t *testing.T) {
	prev := config.RolesModelLister
	config.RolesModelLister = func(context.Context, *config.Root, *config.Provider) ([]string, error) {
		return nil, context.Canceled
	}
	t.Cleanup(func() { config.RolesModelLister = prev })
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning"}},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected unreachable provider error")
	}
}

func TestPendingSubagentSpawnPreservesRoleFields(t *testing.T) {
	in := chatstore.PendingSubagentSpawn{
		SysPromptPath: "agent.tmpl",
		Task:          "explore auth",
		RoleProvider:  "openrouter",
		RoleModel:     "qwen3-32b",
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out chatstore.PendingSubagentSpawn
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.RoleProvider != "openrouter" || out.RoleModel != "qwen3-32b" {
		t.Fatalf("role fields lost: %+v", out)
	}
}

func TestNativeToolParamsIncludesListSubAgents(t *testing.T) {
	params, err := agenttools.NativeToolParams("agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 8 {
		t.Fatalf("agent tools: %d", len(params))
	}
	names := map[string]bool{}
	for _, p := range params {
		if p.OfFunction != nil {
			names[p.OfFunction.Function.Name] = true
		}
	}
	for _, want := range []string{"listSubAgents", "subagent"} {
		if !names[want] {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestRolesScoreWarningsOrphan(t *testing.T) {
	cfg := &config.Root{
		Roles: config.Roles{
			Table: config.RolesTable{Characteristics: []string{"reasoning"}},
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m", Scores: map[string]int{"taste": 70}},
			},
		},
	}
	warns := config.RolesScoreWarnings(cfg)
	if len(warns) != 1 || !strings.Contains(warns[0], "taste") {
		t.Fatalf("warns=%v", warns)
	}
}

func TestFormatSubagentTable(t *testing.T) {
	view := roles.TableView{
		Columns: []string{"cost", "instruction_following", "agentic_capabilities"},
		Rows: []roles.TableRow{
			{Provider: "groq", Model: "llama", Scores: map[string]int{"cost": 80, "instruction_following": 70, "agentic_capabilities": 68}},
		},
	}
	table := roles.FormatSubagentTable(view)
	if !strings.Contains(table, "┌") || !strings.Contains(table, "llama") || !strings.Contains(table, "[groq]") {
		t.Fatalf("table=%q", table)
	}
	if !strings.Contains(table, "💵 cost") || !strings.Contains(table, "📋 instruction following") {
		t.Fatalf("legend missing: %q", table)
	}
	if !strings.Contains(table, "📋") || !strings.Contains(table, "🤖") {
		t.Fatalf("symbol headers missing: %q", table)
	}
	if strings.Contains(table, "│ IF │") || strings.Contains(table, "│ Ag │") {
		t.Fatalf("abbrev headers should be gone: %q", table)
	}
}

func TestCharacteristicColumn(t *testing.T) {
	if roles.CharacteristicColumn("instruction_following") != "📋" {
		t.Fatal("expected instruction_following symbol as column")
	}
	if roles.CharacteristicSymbol("instruction_following") != "📋" {
		t.Fatal("expected instruction_following symbol")
	}
	leg := roles.CharacteristicLegend([]string{"reasoning", "real_cost"})
	if leg != "🧠 reasoning  💰 real cost" {
		t.Fatalf("legend=%q", leg)
	}
}

func TestFormatCompactTable_EmojiHeaderAlignment(t *testing.T) {
	view := roles.TableView{
		Columns: []string{"reasoning", "instruction_following", "real_cost", "agentic_capabilities", "consistency"},
		Rows: []roles.TableRow{
			{Model: "gpt-5-6-sol", Scores: map[string]int{"real_cost": 100, "agentic_capabilities": 100, "consistency": 77}},
		},
	}
	table := roles.FormatCompactTable(view)
	var header string
	for _, line := range strings.Split(table, "\n") {
		if strings.Contains(line, "model") && strings.Contains(line, "🧠") {
			header = line
			break
		}
	}
	if header == "" {
		t.Fatalf("header missing: %q", table)
	}
	parts := strings.Split(header, "│")
	if len(parts) < 7 {
		t.Fatalf("header cells=%d line=%q", len(parts), header)
	}
	for i, want := range []string{"🧠", "📋", "💰", "🤖", "≡"} {
		cell := strings.TrimSpace(parts[i+2])
		if !strings.HasPrefix(cell, want) && cell != want {
			if !strings.Contains(parts[i+2], want) {
				t.Fatalf("col %d missing %q in %q (header=%q)", i, want, parts[i+2], header)
			}
		}
	}
}
