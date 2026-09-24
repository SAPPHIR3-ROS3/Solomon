package cursor

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// AgentTool is a tool made available to Cursor's Agent service by Solomon.
type AgentTool struct {
	Name, Description string
	Schema            []byte
}

type AgentImage struct {
	Data           []byte
	Mime, Filename string
}

type AgentEvent struct {
	Text, Thinking string
	Unknown        string
	Ended          bool
	Tool           *AgentToolCall
	Context        *AgentExecRequest
	MCPState       *AgentExecRequest
	KV             *AgentKVRequest
	Native         *AgentNativeRequest
	Query          *AgentQueryRequest
	Usage          ChatUsage
	Err            error
}

type AgentExecRequest struct {
	ID     uint32
	ExecID string
}
type AgentNativeRequest struct {
	AgentExecRequest
	Kind                                    string
	ResultField, RejectedField, ReasonField int
}
type AgentQueryRequest struct {
	ID   uint32
	Kind int
}
type AgentKVRequest struct {
	ID               uint32
	BlobID, BlobData []byte
	Set              bool
}

type AgentToolCall struct {
	ID, Name, Arguments, ExecID string
	ExecNumber                  uint32
}

type AgentSession struct {
	writer       *io.PipeWriter
	cancel       context.CancelFunc
	events       chan AgentEvent
	mu           sync.Mutex
	closed       bool
	blobs        map[string][]byte
	updatedBlobs []string
}

// OpenAgentSession uses Cursor's subscription Agent Connect endpoint directly.
// The API key is exchanged for a short-lived access token; no local Cursor
// process, CLI config or sidecar participates in the request.
func OpenAgentSession(ctx context.Context, apiKey, model, system, user string, tools []AgentTool, history [][]byte, images []AgentImage) (*AgentSession, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("Cursor Agent: missing subscription API key")
	}
	tokens, err := Refresh(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("Cursor Agent token exchange: %w", err)
	}
	endpoint, err := discoverAgentURL(ctx, tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("Cursor Agent: invalid service endpoint")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/agent.v1.AgentService/Run"
	parsed.RawQuery = ""
	streamCtx, cancel := context.WithCancel(ctx)
	reader, writer := io.Pipe()
	req, err := http.NewRequestWithContext(streamCtx, http.MethodPost, parsed.String(), reader)
	if err != nil {
		cancel()
		reader.Close()
		writer.Close()
		return nil, err
	}
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/connect+proto")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("TE", "trailers")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("X-Ghost-Mode", "true")
	req.Header.Set("X-Cursor-Client-Version", agentClientVersion())
	req.Header.Set("X-Cursor-Client-Type", "cli")
	req.Header.Set("X-Cursor-Streaming", "true")
	req.Header.Set("Connect-Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "connect-es/1.6.1")
	prefix := tokens.AccessToken
	if len(prefix) > 15 {
		prefix = prefix[:15]
	}
	req.Header.Set("Cookie", "CursorCookie=Cookie-"+prefix)
	payload, blobs, err := agentRunRequest(model, system, user, tools, history, images)
	if err != nil {
		cancel()
		reader.Close()
		writer.Close()
		return nil, err
	}
	s := &AgentSession{writer: writer, cancel: cancel, events: make(chan AgentEvent, 64), blobs: blobs}
	go s.readResponse(req)
	if err := s.Write(payload); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func discoverAgentURL(ctx context.Context, access string) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, "https://api2.cursor.sh/aiserver.v1.ServerConfigService/GetServerConfig", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("X-Cursor-Client-Type", "cli")
	req.Header.Set("X-Cursor-Client-Version", agentClientVersion())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Cursor Agent service discovery: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Cursor Agent service discovery HTTP %d: %s", resp.StatusCode, clipChatBody(body))
	}
	var result struct {
		AgentURLConfig struct {
			AgentNURL string `json:"agentnUrl"`
			AgentURL  string `json:"agentUrl"`
		} `json:"agentUrlConfig"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("Cursor Agent service discovery: %w", err)
	}
	endpoint := firstNonEmpty(result.AgentURLConfig.AgentNURL, result.AgentURLConfig.AgentURL)
	if endpoint == "" {
		return "", fmt.Errorf("Cursor Agent service discovery returned no endpoint")
	}
	return endpoint, nil
}

func agentToolWire(tool AgentTool) []byte {
	var def []byte
	def = append(def, pbString(1, tool.Name)...)
	def = append(def, pbString(2, tool.Description)...)
	def = append(def, pbString(4, "solomon")...)
	def = append(def, pbString(5, tool.Name)...)
	def = append(def, pbString(6, string(tool.Schema))...)
	return def
}

func agentRunRequest(model, system, user string, tools []AgentTool, history [][]byte, images []AgentImage) ([]byte, map[string][]byte, error) {
	blobs := make(map[string][]byte, len(history))
	var state []byte
	for _, message := range history {
		if !json.Valid(message) {
			return nil, nil, fmt.Errorf("Cursor Agent history contains invalid JSON")
		}
		id := sha256.Sum256(message)
		blobs[hex.EncodeToString(id[:])] = append([]byte(nil), message...)
		state = append(state, pbBytes(1, id[:])...)
	}
	msg := append(pbString(1, user), pbString(2, newCursorID())...)
	if len(images) > 0 {
		var selected []byte
		for _, image := range images {
			id := newCursorID()
			item := append(pbString(2, id), pbString(3, image.Filename)...)
			item = append(item, pbString(7, image.Mime)...)
			item = append(item, pbBytes(8, image.Data)...)
			selected = append(selected, pbBytes(1, item)...)
		}
		msg = append(msg, pbBytes(3, selected)...)
	}
	action := pbBytes(1, pbBytes(1, msg))
	details := append(pbString(1, model), pbString(3, model)...)
	details = append(details, pbString(4, model)...)
	var run []byte
	run = append(run, pbBytes(1, state)...)
	run = append(run, pbBytes(2, action)...)
	run = append(run, pbBytes(3, details)...)
	if len(tools) > 0 {
		var defs []byte
		for _, tool := range tools {
			defs = append(defs, pbBytes(1, agentToolWire(tool))...)
		}
		run = append(run, pbBytes(4, defs)...)
	}
	run = append(run, pbString(5, newCursorID())...)
	run = append(run, pbBytes(9, pbString(1, model))...)
	_ = system // supplied in RequestContext, not the RunRequest
	return pbBytes(1, run), blobs, nil
}

func (s *AgentSession) Write(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	_, err := s.writer.Write(connectFrame(payload))
	return err
}

func (s *AgentSession) Next(ctx context.Context) (AgentEvent, bool) {
	select {
	case event, ok := <-s.events:
		return event, ok
	case <-ctx.Done():
		return AgentEvent{Err: ctx.Err()}, true
	}
}

// LatestAssistantText returns the text in the newest assistant message Cursor
// persisted during this session. Some Agent responses persist their final
// message in the KV store without sending a text-delta event.
func (s *AgentSession) LatestAssistantText() (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.Lock()
	ids := append([]string(nil), s.updatedBlobs...)
	blobs := make([][]byte, len(ids))
	for i, id := range ids {
		blobs[i] = s.blobs[id]
	}
	s.mu.Unlock()
	for i := len(blobs) - 1; i >= 0; i-- {
		text, isAssistant := assistantMessageText(blobs[i])
		if isAssistant {
			return text, text != ""
		}
	}
	return "", false
}

func (s *AgentSession) Close() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.cancel()
		s.writer.Close()
	}
	s.mu.Unlock()
}

func (s *AgentSession) emit(ctx context.Context, event AgentEvent) bool {
	select {
	case s.events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *AgentSession) readResponse(req *http.Request) {
	defer close(s.events)
	transport := &http2.Transport{}
	defer transport.CloseIdleConnections()
	resp, err := transport.RoundTrip(req)
	if err != nil {
		s.emit(req.Context(), AgentEvent{Err: fmt.Errorf("Cursor Agent transport: %w", err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		s.emit(req.Context(), AgentEvent{Err: fmt.Errorf("Cursor Agent HTTP %d: %s", resp.StatusCode, clipChatBody(body))})
		return
	}
	for {
		var header [5]byte
		if _, err := io.ReadFull(resp.Body, header[:]); err != nil {
			if err != io.EOF {
				s.emit(req.Context(), AgentEvent{Err: fmt.Errorf("Cursor Agent stream: %w", err)})
			}
			return
		}
		size := binary.BigEndian.Uint32(header[1:])
		if size > 16<<20 {
			s.emit(req.Context(), AgentEvent{Err: fmt.Errorf("Cursor Agent frame too large")})
			return
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(resp.Body, payload); err != nil {
			s.emit(req.Context(), AgentEvent{Err: err})
			return
		}
		if header[0]&1 != 0 {
			gz, err := gzip.NewReader(bytes.NewReader(payload))
			if err != nil {
				s.emit(req.Context(), AgentEvent{Err: err})
				return
			}
			payload, err = io.ReadAll(io.LimitReader(gz, 16<<20))
			gz.Close()
			if err != nil {
				s.emit(req.Context(), AgentEvent{Err: err})
				return
			}
		}
		if header[0]&2 != 0 {
			var trailer struct {
				Error *struct {
					Code, Message string
					Details       []struct {
						Type string `json:"type"`
					} `json:"details"`
				} `json:"error"`
			}
			if json.Unmarshal(payload, &trailer) == nil && trailer.Error != nil {
				kinds := make([]string, 0, len(trailer.Error.Details))
				for _, detail := range trailer.Error.Details {
					kinds = append(kinds, detail.Type)
				}
				s.emit(req.Context(), AgentEvent{Err: fmt.Errorf("Cursor Agent: %s: %s (detail types: %v)", trailer.Error.Code, trailer.Error.Message, kinds)})
			}
			return
		}
		if !s.emit(req.Context(), DecodeAgentServerMessage(payload)) {
			return
		}
	}
}

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

func agentClientVersion() string {
	// Cursor's Agent service requires the cli- protocol prefix. This is only a
	// wire identity; Solomon never launches or reads the Cursor CLI.
	version := strings.TrimSpace(os.Getenv("CURSOR_AGENT_CLIENT_VERSION"))
	if version == "" {
		version = "2026.08.25-3e8eec8"
	}
	if !strings.HasPrefix(version, "cli-") {
		version = "cli-" + version
	}
	return version
}
