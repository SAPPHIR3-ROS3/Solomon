package test

import (
	"testing"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
)

func TestRequestedModelCarriesReasoningEffort(t *testing.T) {
	raw := cursorauth.EncodeRequestedModel("cursor-grok-4.7[context=256k,reasoning_effort=medium,fast=true]", []cursorauth.AgentModelParam{
		{ID: "fast", Value: "false"},
		{ID: "reasoning_effort", Value: "low"},
	})
	if got := string(cursorWireField(t, raw, 1)); got != "grok-4.7" {
		t.Fatalf("model=%q", got)
	}
	params := map[string]string{}
	for _, param := range cursorWireFields(t, raw, 3) {
		params[string(cursorWireField(t, param, 1))] = string(cursorWireField(t, param, 2))
	}
	if params["reasoning_effort"] != "low" || params["fast"] != "false" {
		t.Fatalf("params=%v", params)
	}
}

func TestResolveChatModelIDIgnoresParameterBlob(t *testing.T) {
	cursorauth.SetModelCaps(cursorauth.ModelCaps{
		IDs:      []string{"cursor-grok-4.7"},
		Variants: []string{"grok-4.7[context=256k,reasoning_effort=medium,fast=true]", "grok-4.7[context=256k,reasoning_effort=low,fast=false]"},
	})
	if got := cursorauth.ResolveChatModelID("cursor-grok-4.7", "low", true); got != "cursor-grok-4.7" {
		t.Fatalf("selected=%q", got)
	}
}

func TestResolveAgentModelID(t *testing.T) {
	for _, tc := range []struct{ selected, want string }{
		{"cursor-grok-4.7", "grok-4.7"},
		{"cursor-grok-4.7-high", "grok-4.7-high"},
		{"cursor-grok-4.6-medium-fast", "grok-4.6-medium-fast"},
		{"grok-4.7", "grok-4.7"},
		{"composer-2.5", "composer-2.5"},
		{"cursor-other-model", "cursor-other-model"},
		{"  cursor-grok-4.7  ", "grok-4.7"},
		{"grok-4.7[context=256k,reasoning_effort=medium,fast=true]", "grok-4.7"},
		{"cursor-grok-4.7[context=256k,reasoning_effort=low,fast=false]", "grok-4.7"},
	} {
		t.Run(tc.selected, func(t *testing.T) {
			if got := cursorauth.ResolveAgentModelID(tc.selected); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
