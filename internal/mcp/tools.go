package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v2"
)

func (m *Manager) OpenAITools() []openai.ChatCompletionToolUnionParam {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.tools) == 0 {
		return nil
	}
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(m.tools))
	for _, tool := range m.tools {
		if m.serverInternalLocked(tool.ServerName) {
			continue
		}
		out = append(out, OpenAITool(tool))
	}
	return out
}

func (m *Manager) ToolDump() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.tools) == 0 {
		return ""
	}
	var b strings.Builder
	publicIndex := 0
	for _, tool := range m.tools {
		if m.serverInternalLocked(tool.ServerName) {
			continue
		}
		if publicIndex > 0 {
			b.WriteString("\n---\n")
		}
		publicIndex++
		schema, err := json.Marshal(mcpArgumentsSchema(tool.Schema))
		if err != nil {
			schema = []byte(`{"type":"object","properties":{}}`)
		}
		b.WriteString(fmt.Sprintf("name: %s\ndescription: %s\nsdk_call: sdk.mcp.<tool>(intent, args)\nparameters: %s\n", tool.OpenAIName, tool.Description, string(schema)))
	}
	return b.String()
}

func (m *Manager) HasTool(name string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	binding, ok := m.registry[name]
	if ok && binding != nil && binding.server != nil && binding.server.cfg.Internal {
		ok = false
	}
	m.mu.RUnlock()
	return ok
}

func (m *Manager) CallTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if m == nil {
		return nil, fmt.Errorf("MCP manager unavailable")
	}
	if err := tooling.ValidateToolIntent(raw); err != nil {
		return nil, fmt.Errorf("MCP tool %q: %w", name, err)
	}
	if err := m.WaitReady(ctx); err != nil {
		return nil, err
	}
	m.mu.RLock()
	binding, ok := m.registry[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown MCP tool %q", name)
	}
	if binding.server != nil && binding.server.cfg.Internal {
		return nil, fmt.Errorf("MCP tool %q is internal and cannot be called through the generic MCP surface", name)
	}
	return m.callToolBinding(ctx, name, binding, raw)
}

// CallInternalTool invokes a host-managed MCP tool by its stable server and
// remote tool names. Internal adapter calls deliberately bypass the
// model-facing MCP catalog, but still use the Manager's session lifecycle,
// retry policy, and logging.
func (m *Manager) CallInternalTool(ctx context.Context, serverName, toolName, intent string, args map[string]any) (any, error) {
	if m == nil {
		return nil, fmt.Errorf("MCP manager unavailable")
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("MCP internal tool: server name is required")
	}
	if strings.TrimSpace(toolName) == "" {
		return nil, fmt.Errorf("MCP internal tool: tool name is required")
	}
	if strings.TrimSpace(intent) == "" {
		return nil, fmt.Errorf("MCP internal tool %q: intent is required", toolName)
	}
	if err := m.WaitReady(ctx); err != nil {
		return nil, err
	}
	binding, err := m.internalToolBinding(ctx, serverName, toolName)
	if err != nil {
		return nil, err
	}

	callArgs := make(map[string]any, len(args)+1)
	for key, value := range args {
		if key != "intent" {
			callArgs[key] = value
		}
	}
	callArgs["intent"] = intent
	raw, err := json.Marshal(callArgs)
	if err != nil {
		return nil, fmt.Errorf("MCP internal tool %q arguments: %w", toolName, err)
	}
	return m.callToolBinding(ctx, binding.tool.OpenAIName, binding, raw)
}

func (m *Manager) internalToolBinding(ctx context.Context, serverName, toolName string) (*remoteBinding, error) {
	m.mu.RLock()
	var server *serverSession
	for _, candidate := range m.servers {
		if candidate != nil && candidate.cfg.Name == serverName {
			server = candidate
			break
		}
	}
	m.mu.RUnlock()
	if server == nil {
		return nil, fmt.Errorf("unknown internal MCP server %q", serverName)
	}
	if !server.cfg.Internal {
		return nil, fmt.Errorf("MCP server %q is not internal", serverName)
	}
	if _, err := m.ensureSession(ctx, server); err != nil {
		return nil, unavailableError(serverName, "internal tools/call", err)
	}

	lookup := func() *remoteBinding {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, tool := range server.tools {
			if tool.ToolName == toolName {
				toolCopy := tool
				return &remoteBinding{server: server, tool: toolCopy}
			}
		}
		return nil
	}
	if binding := lookup(); binding != nil {
		return binding, nil
	}
	if err := m.refreshTools(ctx, server); err != nil {
		return nil, unavailableError(serverName, "internal tools/list", err)
	}
	if binding := lookup(); binding != nil {
		return binding, nil
	}
	return nil, fmt.Errorf("unknown internal MCP tool %q on server %q", toolName, serverName)
}

func (m *Manager) callToolBinding(ctx context.Context, name string, binding *remoteBinding, raw json.RawMessage) (any, error) {
	if binding == nil {
		return nil, fmt.Errorf("MCP tool %q unavailable", name)
	}
	if binding.server == nil {
		return nil, unavailableError("", "tool", ErrServerUnavailable)
	}
	args := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("MCP tool arguments must be an object: %w", err)
		}
	}
	intent, _ := tooling.ToolIntent(raw)
	delete(args, "intent")
	start := time.Now()
	logFailure := func(err error) (any, error) {
		params := map[string]any{"server": binding.server.cfg.Name, "tool": binding.tool.ToolName, "openai_tool": name, "elapsed_ms": time.Since(start).Milliseconds(), "err": err.Error()}
		if intent != "" {
			params["intent"] = intent
		}
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP tools/call failed", logging.LogOptions{Params: params})
		return nil, err
	}
	call := func(session *sdkmcp.ClientSession) (*sdkmcp.CallToolResult, error) {
		callCtx, cancel := context.WithTimeout(ctx, timeoutFor(binding.server.cfg, m.callTimeout))
		defer cancel()
		return session.CallTool(callCtx, &sdkmcp.CallToolParams{Name: binding.tool.ToolName, Arguments: args})
	}
	session, err := m.ensureSession(ctx, binding.server)
	if err != nil {
		return logFailure(unavailableError(binding.server.cfg.Name, "tools/call", err))
	}
	res, err := call(session)
	if err != nil && isSessionFailure(err) {
		m.invalidateAndCloseSession(binding.server, session, err)
		if m.toolCallMayRetry(binding.server.cfg.Name, binding.tool.ToolName, binding.tool.Definition, err) {
			session, reconnectErr := m.ensureSession(ctx, binding.server)
			if reconnectErr != nil {
				return logFailure(unknownOutcomeError(name, errors.Join(err, reconnectErr)))
			}
			res, err = call(session)
			if err != nil && isSessionFailure(err) {
				m.invalidateAndCloseSession(binding.server, session, err)
				return logFailure(unknownOutcomeError(name, err))
			}
		} else {
			// Reconnect for the next operation, but never replay a call whose
			// delivery status is uncertain (including multi-round-trip calls).
			if _, reconnectErr := m.ensureSession(ctx, binding.server); reconnectErr != nil {
				err = errors.Join(err, reconnectErr)
			}
			return logFailure(unknownOutcomeError(name, err))
		}
	}
	elapsed := time.Since(start)
	params := map[string]any{"server": binding.server.cfg.Name, "tool": binding.tool.ToolName, "openai_tool": name, "elapsed_ms": elapsed.Milliseconds()}
	if intent != "" {
		params["intent"] = intent
	}
	if err != nil {
		params["err"] = err.Error()
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP tools/call failed", logging.LogOptions{Params: params})
		return nil, err
	}
	logging.Log(logging.INFO_LOG_LEVEL, "MCP tools/call completed", logging.LogOptions{Params: params})
	return convertResult(res)
}

func convertResult(res *sdkmcp.CallToolResult) (any, error) {
	if res == nil {
		return nil, nil
	}
	content := contentValues(res.Content)
	if res.NeedsInput() {
		return map[string]any{
			"resultType":    "input_required",
			"inputRequests": res.InputRequests,
			"requestState":  res.RequestState,
			"content":       content,
		}, nil
	}
	if res.IsError {
		return map[string]any{"error": contentText(content)}, nil
	}
	if res.StructuredContent != nil {
		return map[string]any{"structuredContent": res.StructuredContent, "content": content}, nil
	}
	if len(content) == 1 {
		return content[0], nil
	}
	return content, nil
}

func contentValues(items []sdkmcp.Content) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case *sdkmcp.TextContent:
			out = append(out, v.Text)
		default:
			b, err := item.MarshalJSON()
			if err != nil {
				out = append(out, map[string]any{"error": err.Error()})
				continue
			}
			var decoded any
			if err := json.Unmarshal(b, &decoded); err != nil {
				out = append(out, string(b))
				continue
			}
			out = append(out, decoded)
		}
	}
	return out
}

func contentText(items []any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			parts = append(parts, s)
			continue
		}
		b, _ := json.Marshal(item)
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n")
}
