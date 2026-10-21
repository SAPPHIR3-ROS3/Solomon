package cursor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
)

func (s *AgentSession) RejectNative(request AgentNativeRequest) error {
	if strings.HasPrefix(request.Kind, "precheck_") {
		var resultField int
		switch request.Kind {
		case "precheck_shell":
			resultField = 41
		case "precheck_mcp":
			resultField = 42
		case "precheck_web_fetch":
			resultField = 43
		default:
			return fmt.Errorf("Cursor Agent: unsupported precheck %q", request.Kind)
		}
		exec := append(pbVarint(1, uint64(request.ID)), pbBytes(resultField, nil)...)
		exec = append(exec, pbString(15, request.ExecID)...)
		return s.Write(pbBytes(2, exec))
	}
	var result []byte
	reason := "Use Solomon's provided MCP tools for workspace operations."
	if request.RejectedField > 0 {
		result = pbBytes(request.RejectedField, pbString(request.ReasonField, reason))
	} else if request.Kind == "fetch" {
		result = pbBytes(2, pbString(2, reason))
	} else {
		result = pbBytes(2, pbString(1, reason))
	}
	exec := append(pbVarint(1, uint64(request.ID)), pbBytes(request.ResultField, result)...)
	exec = append(exec, pbString(15, request.ExecID)...)
	return s.Write(pbBytes(2, exec))
}

func (s *AgentSession) RejectQuery(query AgentQueryRequest) error {
	if query.Kind == 0 {
		return fmt.Errorf("Cursor Agent requested unknown interaction")
	}
	reason := "Interactive native tools are unavailable; use Solomon's provided MCP tools."
	var result []byte
	switch query.Kind {
	case 2, 4, 5, 6, 9:
		result = pbBytes(2, pbString(1, reason))
	case 3:
		result = pbBytes(1, pbBytes(3, pbString(1, reason)))
	case 7:
		result = pbBytes(1, pbBytes(2, pbString(1, reason)))
	default:
		return fmt.Errorf("Cursor Agent requested unsupported interaction %d", query.Kind)
	}
	response := append(pbVarint(1, uint64(query.ID)), pbBytes(query.Kind, result)...)
	return s.Write(pbBytes(6, response))
}

func (s *AgentSession) MCPStateResult(request AgentExecRequest, tools []AgentTool) error {
	var definitions []byte
	for _, tool := range tools {
		schema := string(tool.Schema)
		definition := append(pbString(1, tool.Name), pbString(2, tool.Description)...)
		definition = append(definition, pbString(4, "solomon")...)
		definition = append(definition, pbString(5, tool.Name)...)
		definition = append(definition, pbString(6, schema)...)
		definitions = append(definitions, pbBytes(5, definition)...)
	}
	server := append(pbString(1, "solomon"), pbString(2, "solomon")...)
	server = append(server, definitions...)
	server = append(server, pbString(7, "connected")...)
	success := pbBytes(1, pbBytes(1, server))
	exec := append(pbVarint(1, uint64(request.ID)), pbBytes(36, success)...)
	exec = append(exec, pbString(15, request.ExecID)...)
	return s.Write(pbBytes(2, exec))
}

func (s *AgentSession) KVResult(request AgentKVRequest) error {
	if len(request.BlobID) != sha256.Size {
		return fmt.Errorf("Cursor Agent blob identifier has invalid length")
	}
	key := hex.EncodeToString(request.BlobID)
	if request.Set {
		if len(request.BlobData) > 8<<20 {
			return fmt.Errorf("Cursor Agent blob exceeds size limit")
		}
		s.mu.Lock()
		if s.blobs == nil {
			s.blobs = make(map[string][]byte)
		}
		s.blobs[key] = append([]byte(nil), request.BlobData...)
		s.updatedBlobs = append(s.updatedBlobs, key)
		s.mu.Unlock()
		return s.Write(pbBytes(3, append(pbVarint(1, uint64(request.ID)), pbBytes(3, nil)...)))
	}
	var result []byte
	s.mu.Lock()
	if blob, ok := s.blobs[key]; ok {
		result = pbBytes(1, append([]byte(nil), blob...))
	}
	s.mu.Unlock()
	return s.Write(pbBytes(3, append(pbVarint(1, uint64(request.ID)), pbBytes(2, result)...)))
}

func (s *AgentSession) ContextResult(id uint32, execID, system string, tools []AgentTool) error {
	env := pbString(1, runtime.GOOS)
	context := pbBytes(4, env)
	for _, tool := range tools {
		context = append(context, pbBytes(7, agentToolWire(tool))...)
	}
	if system != "" {
		context = append(context, pbString(16, system)...)
	}
	result := pbBytes(1, pbBytes(1, context))
	exec := append(pbVarint(1, uint64(id)), pbBytes(10, result)...)
	exec = append(exec, pbString(15, execID)...)
	return s.Write(pbBytes(2, exec))
}

func (s *AgentSession) ToolResult(call AgentToolCall, content string, isError bool) error {
	item := pbBytes(1, pbString(1, content))
	success := pbBytes(1, item)
	if isError {
		success = append(success, pbVarint(2, 1)...)
	}
	exec := append(pbVarint(1, uint64(call.ExecNumber)), pbBytes(11, pbBytes(1, success))...)
	exec = append(exec, pbString(15, call.ExecID)...)
	return s.Write(pbBytes(2, exec))
}
