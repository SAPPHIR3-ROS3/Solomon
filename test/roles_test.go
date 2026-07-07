package test

import (
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
			return []string{"qwen"}, nil
		case "groq":
			return []string{"llama"}, nil
		default:
			return []string{"m", "qwen", "llama", "other"}, nil
		}
	}
	t.Cleanup(func() { config.RolesModelLister = prev })
}

func testRolesConfig() *config.Root {
	return &config.Root{
		Roles: config.Roles{
			Subagent: []config.SubagentRoleConfig{
				{Provider: "openrouter", Model: "qwen", Description: "explore", Points: 80},
				{Provider: "groq", Model: "llama", Description: "cheap", Points: 60},
			},
		},
	}
}

func TestRolesSubagentPoolSortedByPoints(t *testing.T) {
	pool := roles.SubagentPool(testRolesConfig())
	if len(pool) != 2 || pool[0].Model != "qwen" || pool[1].Model != "llama" {
		t.Fatalf("pool: %+v", pool)
	}
}

func TestRolesSubagentDefaultPoints(t *testing.T) {
	cfg := &config.Root{
		Roles: config.Roles{
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m", Description: "x"},
			},
		},
	}
	pool := roles.SubagentPool(cfg)
	if len(pool) != 1 || pool[0].Points != config.DefaultSubagentRolePoints {
		t.Fatalf("points: %+v", pool[0])
	}
}

func TestRolesFindSubagent(t *testing.T) {
	_, err := roles.FindSubagent(testRolesConfig(), "openrouter", "missing")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	e, err := roles.FindSubagent(testRolesConfig(), "openrouter", "qwen")
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

[[roles.subagent]]
model = "qwen"
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

[[roles.subagent]]
provider = "p"
model = "m"
description = "explore"
points = 80
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
			Subagent: []config.SubagentRoleConfig{
				{Provider: "q", Model: "other", Points: 50},
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
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m", Points: 80},
				{Provider: "p", Model: "m", Points: 60},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate pair error")
	}
}

func TestValidateRolesRejectsNegativePoints(t *testing.T) {
	withMockRolesModelLister(t)
	err := config.ValidateRoles(context.Background(), &config.Root{
		Providers: map[string]*config.Provider{"p": {Name: "p"}},
		Current:   config.Current{Provider: "p", Model: "m"},
		Roles: config.Roles{
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m", Points: -1},
			},
		},
	})
	if err == nil {
		t.Fatal("expected negative points error")
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
			Subagent: []config.SubagentRoleConfig{
				{Provider: "p", Model: "m"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected unreachable provider error")
	}
}

func TestConfigRolesValidationRejectsUnknownModelOnLoad(t *testing.T) {
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

[[roles.subagent]]
provider = "p"
model = "missing-model"
`)
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected validation error for unknown model")
	}
}

func TestPendingSubagentSpawnPreservesRoleFields(t *testing.T) {
	in := chatstore.PendingSubagentSpawn{
		SysPromptPath: "agent.tmpl",
		Task:          "explore auth",
		RoleProvider:  "openrouter",
		RoleModel:     "qwen",
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out chatstore.PendingSubagentSpawn
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.RoleProvider != "openrouter" || out.RoleModel != "qwen" {
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
