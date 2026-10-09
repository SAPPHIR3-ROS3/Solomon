package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

const readChatFixture = `{"id":"reference-chat","title":"Prior decisions","created_at":"2026-10-01T12:00:00Z","messages":[
 {"role":"system","content":"hidden system instructions"},
 {"role":"user","content":"Requirements","created_at":"2026-10-01T12:00:01Z","user_prompt_tokens":12},
 {"role":"assistant","content":"<think>hidden reasoning</think>Checking","api_content":"hidden api content","reasoning_text":"hidden reasoning","tool_calls":[{"id":"call-1","name":"shell","arguments":"{\"command\":\"pwd\"}"}],"prompt_tokens":9007199254740993,"cached_prompt_tokens":90,"reasoning_tokens":3,"response_tokens":7,"turn_total_tokens":110,"output_tps":4.5,"ttft_secs":0.2,"turn_wall_secs":1.2,"turn_display_saved":true,"turn_context_tokens":100,"turn_context_est":false,"turn_total_display":110},
 {"role":"tool","tool_call_id":"call-1","content":"tool output","turn_wall_secs":0.1},
 {"role":"assistant","content":"Decision: use X"}
 ],"branches":[{"fork_at":1,"messages":[{"role":"user","content":"hidden branch"}]}],"uncompactedRaw":[{"messages":[{"role":"user","content":"hidden archive"}]}]}`

func savedReadChat(t *testing.T, home, projectID, subdir, data string) string {
	t.Helper()
	p := filepath.Join(home, "projects", projectID, "chats", subdir, "reference-chat.json")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func runReadChat(t *testing.T, mode string, args map[string]any) (map[string]json.RawMessage, error) {
	t.Helper()
	args["intent"] = "Read the referenced conversation"
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tools.Exec(t.Context(), &tools.Env{ProjHex: "current-project"}, mode, tooling.Invocation{Name: "readChat", Args: raw})
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	return result, nil
}

func TestReadChatFlagsAndCrossProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	p := savedReadChat(t, home, "other-project", "", readChatFixture)
	projectRoot := filepath.Join(home, "other", "workspace")
	projectIDs, err := json.Marshal(map[string]string{projectRoot: "other-project"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "projectsId.json"), projectIDs, 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"agent", "chat"} {
		for _, withTools := range []bool{false, true} {
			for _, withStats := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/tools=%t/stats=%t", mode, withTools, withStats), func(t *testing.T) {
					result, err := runReadChat(t, mode, map[string]any{"chatId": "reference-chat", "tools": withTools, "stats": withStats})
					if err != nil {
						t.Fatal(err)
					}
					var gotProjectRoot string
					if err := json.Unmarshal(result["projectRoot"], &gotProjectRoot); err != nil {
						t.Fatal(err)
					}
					if string(result["projectId"]) != `"other-project"` || gotProjectRoot != projectRoot || string(result["compacted"]) != "true" {
						t.Fatalf("metadata: %s", result)
					}
					var msgs []map[string]json.RawMessage
					if err := json.Unmarshal(result["messages"], &msgs); err != nil {
						t.Fatal(err)
					}
					want := 3
					if withTools {
						want = 4
					}
					if len(msgs) != want {
						t.Fatalf("messages=%d want %d", len(msgs), want)
					}
					if string(msgs[0]["index"]) != "1" || string(msgs[0]["created_at"]) != `"2026-10-01T12:00:01Z"` {
						t.Fatalf("lost original index/timestamp: %s", msgs[0])
					}
					for _, msg := range msgs {
						_, hasStats := msg["stats"]
						expectedStats := withStats && string(msg["content"]) != `"Decision: use X"`
						if hasStats != expectedStats {
							t.Fatalf("stats present=%t expected=%t: %s", hasStats, expectedStats, msg)
						}
						if !withTools {
							if _, ok := msg["tool_calls"]; ok {
								t.Fatal("tool calls leaked")
							}
							if _, ok := msg["tool_call_id"]; ok {
								t.Fatal("tool result ID leaked")
							}
						}
					}
					if withTools {
						if len(msgs[1]["tool_calls"]) == 0 || string(msgs[2]["tool_call_id"]) != `"call-1"` || string(msgs[2]["content"]) != `"tool output"` {
							t.Fatal("tool correlation lost")
						}
					}
					if withStats {
						var stats map[string]json.RawMessage
						if err := json.Unmarshal(msgs[1]["stats"], &stats); err != nil {
							t.Fatal(err)
						}
						if string(stats["prompt_tokens"]) != "9007199254740993" || string(stats["ttft_secs"]) != "0.2" || string(stats["turn_context_est"]) != "false" {
							t.Fatalf("statistics changed: %s", stats)
						}
					}
					body, _ := json.Marshal(result)
					for _, secret := range []string{"hidden reasoning", "hidden system", "hidden api", "hidden branch", "hidden archive"} {
						if strings.Contains(string(body), secret) {
							t.Fatalf("leaked %s", secret)
						}
					}
				})
			}
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != readChatFixture {
		t.Fatal("readChat modified saved chat")
	}
}

func TestReadChatDefaultsAndSubchat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	savedReadChat(t, home, "unregistered-project", "subchats", readChatFixture)
	result, err := runReadChat(t, "chat", map[string]any{"chatId": "reference-chat"})
	if err != nil {
		t.Fatal(err)
	}
	var msgs []map[string]json.RawMessage
	json.Unmarshal(result["messages"], &msgs)
	if len(msgs) != 3 {
		t.Fatalf("default messages=%d", len(msgs))
	}
	for _, msg := range msgs {
		if msg["stats"] != nil || msg["tool_calls"] != nil || msg["tool_call_id"] != nil {
			t.Fatalf("defaults leaked optional data: %s", msg)
		}
	}
}

func TestReadChatLegacyTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	data := `{"id":"reference-chat","messages":[{"role":"assistant","content":"Before <tool_calls><tool name=\"shell\"><intent>Inspect</intent><args>{\"command\":\"pwd\"}</args></tool></tool_calls> After"},{"role":"user","content":"tool_result(legacy output)"}]}`
	savedReadChat(t, home, "project", "", data)
	for _, include := range []bool{false, true} {
		result, err := runReadChat(t, "agent", map[string]any{"chatId": "reference-chat", "tools": include})
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(result)
		if strings.Contains(string(b), "legacy output") != include || strings.Contains(string(b), "tool_calls") != include {
			t.Fatalf("legacy tools filtering failed: %s", b)
		}
		if !strings.Contains(string(b), "Before") || !strings.Contains(string(b), "After") {
			t.Fatal("prose lost")
		}
	}
}

func TestReadChatErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  map[string]any
		setup string
		want  string
	}{
		{"missing ID", map[string]any{}, "", "invalid chatId"},
		{"empty ID", map[string]any{"chatId": " "}, "", "invalid chatId"},
		{"traversal", map[string]any{"chatId": "../outside"}, "", "invalid chatId"},
		{"backslash", map[string]any{"chatId": `..\outside`}, "", "invalid chatId"},
		{"bad flag", map[string]any{"chatId": "reference-chat", "stats": "yes"}, "", "cannot unmarshal"},
		{"missing archive", map[string]any{"chatId": "reference-chat"}, "", "not found"},
		{"missing chat", map[string]any{"chatId": "absent"}, "valid", "not found"},
		{"ambiguous", map[string]any{"chatId": "reference-chat"}, "duplicate", "ambiguous"},
		{"corrupt", map[string]any{"chatId": "reference-chat"}, "corrupt", "invalid saved chat"},
		{"mismatched ID", map[string]any{"chatId": "reference-chat"}, "mismatch", "does not match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("SOLOMON_HOME", home)
			switch tc.setup {
			case "valid":
				savedReadChat(t, home, "project", "", readChatFixture)
			case "duplicate":
				savedReadChat(t, home, "a", "", readChatFixture)
				savedReadChat(t, home, "b", "", readChatFixture)
			case "corrupt":
				savedReadChat(t, home, "project", "", `{`)
			case "mismatch":
				savedReadChat(t, home, "project", "", `{"id":"different"}`)
			}
			_, err := runReadChat(t, "agent", tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %s", err, tc.want)
			}
		})
	}
}

func TestReadChatCancellation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	savedReadChat(t, home, "project", "", readChatFixture)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := tools.Exec(ctx, &tools.Env{}, "agent", tooling.Invocation{Name: "readChat", Args: json.RawMessage(`{"chatId":"reference-chat","intent":"Read chat"}`)})
	if err != context.Canceled {
		t.Fatalf("error=%v", err)
	}
}

func TestReadChatExposure(t *testing.T) {
	for _, mode := range []string{"agent", "chat"} {
		params, err := tools.NativeToolParams(mode)
		if err != nil {
			t.Fatal(err)
		}
		params = tools.EnsureUniversalTools(tools.EnsureUniversalTools(params))
		if params[0].OfFunction.Function.Name != "docsRetrieval" {
			t.Fatal("universal order changed")
		}
		count := 0
		for _, p := range params {
			if p.OfFunction != nil && p.OfFunction.Function.Name == "readChat" {
				count++
				schema := p.OfFunction.Function.Parameters
				props := schema["properties"].(map[string]any)
				for _, flag := range []string{"tools", "stats"} {
					if props[flag].(map[string]any)["default"] != false {
						t.Fatal("wrong flag default")
					}
				}
				required := schema["required"].([]any)
				if fmt.Sprint(required) != "[chatId intent]" {
					t.Fatalf("required=%v", required)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s count=%d", mode, count)
		}
	}
	for _, build := range []func() (string, error){tools.BuildAgentToolDump, tools.BuildChatToolDump} {
		dump, err := build()
		if err != nil || !strings.Contains(dump, "name: readChat") {
			t.Fatalf("dump missing readChat: %v", err)
		}
	}
}

func TestReadChatSDKRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	savedReadChat(t, home, "project", "", readChatFixture)
	source := `package main
 import("fmt";"sdk")
 func main(){out,err:=sdk.ReadChat("reference-chat",true,true,"Read prior chat");if err!=nil{panic(err)};fmt.Print(out)}`
	wasm, err := compile.BuildWASM(compile.Options{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	exec := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
		if name != "readChat" {
			return nil, fmt.Errorf("unexpected tool %s", name)
		}
		out, err := tools.Exec(ctx, &tools.Env{AllowDeferredTools: true}, "agent", tooling.Invocation{Name: name, Args: args})
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	}
	done, err := parent.RunGlobal(t.Context(), wasm, "agent", exec)
	if err != nil {
		t.Fatal(err)
	}
	if done.Error != "" || !strings.Contains(done.Output, `"tool output"`) || !strings.Contains(done.Output, `"stats"`) {
		t.Fatalf("SDK result=%+v", done)
	}
}
