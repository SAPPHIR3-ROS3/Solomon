package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/acp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type acpRunner struct {
	runtime *agentruntime.Runtime
	models  []string
}

func newACPRunner(ctx context.Context, cwd string, servers []json.RawMessage) (acp.Runner, error) {
	cfg, err := config.LoadOptional()
	if err != nil {
		return nil, err
	}
	cfg, prov, err := config.ResolveExecConfig(cfg, config.ExecResolveOpts{})
	if err != nil {
		return nil, &acp.Error{Code: -32000, Message: err.Error()}
	}
	root, hash, err := project.Resolve(cwd)
	if err != nil {
		return nil, err
	}
	sessionID := make([]byte, 16)
	if _, err := rand.Read(sessionID); err != nil {
		return nil, err
	}
	sess := &chatstore.Session{ID: hex.EncodeToString(sessionID), CreatedAt: time.Now(), LastMessageAt: time.Now(), CheckpointLast: -1, CheckpointCP0: true}
	rt := agentruntime.NewRuntime(nil, cfg, prov, hash, root, sess)
	ready := false
	defer func() {
		if !ready {
			_ = rt.Close()
		}
	}()
	rt.EphemeralSession = true
	if err := rt.WaitReady(ctx); err != nil {
		return nil, err
	}
	if rt.Backend == nil {
		return nil, fmt.Errorf("provider backend initialization failed; see stderr")
	}
	if len(servers) == 0 {
		rt.InitMCP(ctx)
	} else {
		mc, err := solomonmcp.LoadConfig()
		if err != nil {
			return nil, err
		}
		specs, err := acp.ParseMCP(servers)
		if err != nil {
			return nil, err
		}
		requested := map[string]bool{}
		for _, item := range specs {
			requested[item.Name] = true
		}
		local := mc.Servers[:0]
		for _, item := range mc.Servers {
			if !requested[item.Name] {
				local = append(local, item)
			}
		}
		mc.Servers = local
		for _, item := range specs {
			if item.Type == "http" || item.Type == "sse" {
				if item.URL == "" {
					return nil, fmt.Errorf("MCP server URL is required")
				}
				headers := map[string]string{}
				for _, h := range item.Headers {
					headers[h.Name] = h.Value
				}
				kind := solomonmcp.TransportStreamableHTTP
				if item.Type == "sse" {
					kind = solomonmcp.TransportSSE
				}
				mc.Servers = append(mc.Servers, solomonmcp.ServerConfig{Name: item.Name, Type: kind, URL: item.URL, Headers: headers, CWD: cwd})
				continue
			}
			if item.Command == "" || (item.Type != "" && item.Type != "stdio") {
				return nil, fmt.Errorf("invalid stdio MCP server")
			}

			env := map[string]string{}
			for _, e := range item.Env {
				env[e.Name] = e.Value
			}
			mc.Servers = append(mc.Servers, solomonmcp.ServerConfig{Name: item.Name, Type: "stdio", Command: item.Command, Args: item.Args, Env: env, CWD: cwd})
		}
		rt.SetMCPManager(solomonmcp.NewManager(ctx, mc, os.Stderr))
		if err := rt.MCP.WaitReady(ctx); err != nil {
			return nil, err
		}
		connected := map[string]bool{}
		for _, info := range rt.MCP.Servers() {
			connected[info.Name] = true
		}
		for _, spec := range specs {
			if !connected[spec.Name] {
				return nil, fmt.Errorf("MCP server %s did not connect", spec.Name)
			}
		}
	}
	ready = true
	return &acpRunner{runtime: rt, models: discoverACPModels(ctx, cfg)}, nil
}
func (r *acpRunner) Close() error { return r.runtime.Close() }
func (r *acpRunner) Prompt(ctx context.Context, text string, emit func(acp.Update)) error {
	r.runtime.LastFinishReason = ""
	if strings.TrimSpace(text) == "/help" {
		emit(acp.Update{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "Solomon ACP supports text, images, embedded resources, tool approvals, session history, model selection, and reasoning configuration. Use /compact to summarize the conversation."}})
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(text), "/") && strings.TrimSpace(text) != "/compact" {
		emit(acp.Update{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "Unknown ACP command. Available commands: /help, /compact."}})
		return nil
	}
	r.runtime.EventSink = &acpSink{emit: emit}
	err := r.runtime.RunPromptOnce(ctx, text)
	if err == nil && strings.TrimSpace(text) == "/compact" {
		emit(acp.Update{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "Conversation compacted."}})
	}
	return err
}

type acpSink struct {
	emit      func(acp.Update)
	content   bool
	reasoning bool
}

func (s *acpSink) StreamMode() bool                                                { return true }
func (s *acpSink) Events() []cievents.Event                                        { return nil }
func (s *acpSink) FlushReport(cievents.ReportMeta, int, string, string, any) error { return nil }
func (s *acpSink) Emit(e cievents.Event) {
	switch e["type"] {
	case cievents.TypeAssistantStart:
		s.content = false
		s.reasoning = false
	case cievents.TypeAssistantEnd:
		for _, item := range []struct {
			key, kind string
			sent      bool
		}{{"content", "agent_message_chunk", s.content}, {"reasoning", "agent_thought_chunk", s.reasoning}} {
			if text, ok := e[item.key].(string); ok && text != "" && !item.sent {
				s.emit(acp.Update{"sessionUpdate": item.kind, "content": map[string]any{"type": "text", "text": text}})
			}
		}
	case cievents.TypeAssistantDelta:
		if e["channel"] == cievents.ChannelReasoning {
			s.reasoning = true
		} else {
			s.content = true
		}
		kind := "agent_message_chunk"
		if e["channel"] == cievents.ChannelReasoning {
			kind = "agent_thought_chunk"
		}
		s.emit(acp.Update{"sessionUpdate": kind, "content": map[string]any{"type": "text", "text": e["delta"]}})
	case cievents.TypeToolStart:
		s.emit(acp.Update{"sessionUpdate": "tool_call", "toolCallId": e["id"], "title": e["name"], "name": e["name"], "kind": "other", "status": "in_progress", "rawInput": e["arguments"]})
	case cievents.TypeToolResult:
		status := "completed"
		if e["error"] != nil {
			status = "failed"
		}
		output, _ := json.Marshal(e["result"])
		s.emit(acp.Update{"sessionUpdate": "tool_call_update", "toolCallId": e["id"], "status": status, "rawOutput": e["result"], "content": []any{map[string]any{"type": "content", "content": map[string]any{"type": "text", "text": string(output)}}}})
	}
}
func runACP() error {
	if len(os.Args) == 3 && os.Args[2] == "--icon" {
		_, err := fmt.Fprintln(os.Stdout, logo.MarkSVG())
		return err
	}
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: solomon acp [--icon]")
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	protocolOut := os.Stdout
	os.Stdout = os.Stderr
	termcolor.Init(termcolor.InitOptions{Out: os.Stderr, NoColor: true})
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	server := acp.New(protocolOut, newACPRunner)
	server.SetVersion(commands.LocalVersionString())
	if err = server.SetStorage(filepath.Join(home, "acp", "sessions")); err != nil {
		return err
	}
	return server.Serve(context.Background(), os.Stdin)
}
