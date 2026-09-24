package cursor

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const chatURL = "https://api2.cursor.sh/aiserver.v1.InferenceService/Stream"

var chatEndpoints = []string{chatURL}

type ChatTurn struct {
	Role       string
	Content    string
	ToolCalls  []ChatToolCall
	ToolCallID string
	ToolName   string
	ToolError  bool
}

// ChatTool is the provider-neutral tool description used by the Go Cursor
// client. Cursor's current inference API receives these as protobuf Structs.
type ChatTool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ChatToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ChatResult struct {
	Content      string
	ToolCalls    []ChatToolCall
	FinishReason string
	Usage        ChatUsage
}

type ChatUsage struct {
	PromptTokens       int64
	CompletionTokens   int64
	TotalTokens        int64
	CachedPromptTokens int64
	CacheWriteTokens   int64
	ReasoningTokens    int64
}

func SetChatEndpointForTest(url string) func() {
	old := chatEndpoints
	chatEndpoints = []string{url}
	return func() { chatEndpoints = old }
}

func StreamChat(ctx context.Context, accessToken, model string, turns []ChatTurn, out io.Writer) (string, error) {
	result, err := StreamChatWithTools(ctx, accessToken, model, turns, nil, out)
	return result.Content, err
}

func StreamChatWithTools(ctx context.Context, accessToken, model string, turns []ChatTurn, tools []ChatTool, out io.Writer) (ChatResult, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return ChatResult{}, fmt.Errorf("cursor chat: missing session token")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	endpoints := chatEndpoints
	if override := strings.TrimSpace(os.Getenv("CURSOR_CHAT_ENDPOINT")); override != "" && (len(chatEndpoints) == 0 || chatEndpoints[0] == chatURL) {
		endpoints = []string{override}
	}
	var last error
	for _, ep := range endpoints {
		request, err := buildCursorChatRequest(ctx, accessToken, model, turns, tools, ep)
		if err != nil {
			last = err
			continue
		}
		result, err := postCursorChat(ctx, accessToken, ep, connectFrame(request), out)
		if err == nil {
			return result, nil
		}
		last = err
	}
	return ChatResult{}, last
}

func postCursorChat(ctx context.Context, accessToken, endpoint string, body []byte, out io.Writer) (ChatResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, err
	}
	reqID := newCursorID()
	applyCursorClientHeaders(req, accessToken, reqID)
	req.Header.Set("Content-Type", "application/connect+proto")
	req.Header.Set("Connect-Accept-Encoding", "gzip")
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ForceAttemptHTTP2 = true
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ChatResult{}, fmt.Errorf("cursor chat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		raw = maybeGunzip(raw)
		if msg := jsonChatError(raw); msg != "" {
			return ChatResult{}, fmt.Errorf("cursor chat status %d: %s", resp.StatusCode, msg)
		}
		return ChatResult{}, fmt.Errorf("cursor chat status %d: %s", resp.StatusCode, clipChatBody(raw))
	}
	if usesUnifiedChat(endpoint) {
		return readUnifiedChatResponse(resp.Body, out)
	}
	return readInferenceResponse(resp.Body, out)
}

func buildCursorChatRequest(ctx context.Context, accessToken, model string, turns []ChatTurn, tools []ChatTool, endpoint string) ([]byte, error) {
	if usesUnifiedChat(endpoint) {
		name := CanonicalCursorModelID(normalizeChatModel(model))
		if os.Getenv("CURSOR_SKIP_MODEL_RESOLVE") == "" {
			if selection, err := resolveInferenceModel(ctx, accessToken, model); err == nil {
				name = firstNonEmpty(selection.Display, name)
			}
		}
		body := encodeUnifiedChatRequest(name, turns, tools)
		if strings.Contains(endpoint, "WithTools") {
			return pbBytes(1, body), nil
		}
		return body, nil
	}
	var (
		selection inferenceModel
		err       error
	)
	if os.Getenv("CURSOR_SKIP_MODEL_RESOLVE") != "" {
		selection = inferenceModel{ID: normalizeChatModel(model)}
	} else {
		selection, err = resolveInferenceModel(ctx, accessToken, model)
		if err != nil {
			return nil, err
		}
	}
	return encodeInferenceRequest(selection, turns, tools), nil
}

func inferenceAccessToken(ctx context.Context, session string) string {
	session = strings.TrimSpace(session)
	if strings.Count(session, ".") != 2 || len(session) < 80 {
		return session
	}
	apiAccessMu.Lock()
	if apiAccessSession == session && apiAccessToken != "" && time.Now().Before(apiAccessUntil) {
		tok := apiAccessToken
		apiAccessMu.Unlock()
		return tok
	}
	apiAccessMu.Unlock()
	key, err := MintUserAPIKey(ctx, session)
	if err != nil {
		return session
	}
	tokens, err := Refresh(ctx, key)
	if err != nil || strings.TrimSpace(tokens.AccessToken) == "" {
		return session
	}
	apiAccessMu.Lock()
	apiAccessSession = session
	apiAccessToken = tokens.AccessToken
	apiAccessUntil = time.Now().Add(20 * time.Minute)
	if !tokens.ExpiresAt.IsZero() && tokens.ExpiresAt.Before(apiAccessUntil) {
		apiAccessUntil = tokens.ExpiresAt
	}
	apiAccessMu.Unlock()
	return tokens.AccessToken
}

var (
	apiAccessMu      sync.Mutex
	apiAccessSession string
	apiAccessToken   string
	apiAccessUntil   time.Time
)

func applyCursorClientHeaders(req *http.Request, accessToken, reqID string) {
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("User-Agent", "connect-es/1.6.1")
	req.Header.Set("x-amzn-trace-id", "Root="+reqID)
	if os.Getenv("CURSOR_SKIP_CHECKSUM") == "" {
		req.Header.Set("x-cursor-checksum", cursorChecksum(accessToken))
	}
	if key := strings.TrimSpace(os.Getenv("CURSOR_CLIENT_KEY")); key != "" {
		req.Header.Set("x-client-key", key)
	}
	req.Header.Set("x-cursor-client-version", cursorClientVersionHeader())
	req.Header.Set("x-cursor-client-commit", cursorClientCommitHeader())
	req.Header.Set("x-cursor-client-type", "ide")
	req.Header.Set("x-cursor-client-os", cursorClientOS())
	req.Header.Set("x-cursor-client-arch", cursorClientArch())
	req.Header.Set("x-cursor-client-os-version", cursorClientOSVersion())
	req.Header.Set("x-cursor-client-device-type", "desktop")
	req.Header.Set("x-cursor-client-layout", "editor")
	req.Header.Set("x-ghost-mode", "false")
	req.Header.Set("x-new-onboarding-completed", "false")
	req.Header.Set("x-cursor-timezone", "UTC")
	req.Header.Set("x-request-id", reqID)
}

func usesUnifiedChat(endpoint string) bool {
	if strings.Contains(endpoint, "InferenceService") {
		return false
	}
	if os.Getenv("CURSOR_LEGACY_CHAT") != "" {
		return true
	}
	return strings.Contains(endpoint, "ChatService")
}

func normalizeChatModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "composer-2.5"
	}
	return model
}

func jsonChatError(raw []byte) string {
	var payload any
	if json.Unmarshal(raw, &payload) != nil {
		for _, frame := range connectJSONPayloads(raw) {
			if msg := jsonChatError([]byte(frame)); msg != "" {
				return msg
			}
		}
		return ""
	}
	return findJSONError(payload)
}

func connectJSONPayloads(raw []byte) []string {
	var out []string
	buf := raw
	for len(buf) >= 5 {
		n := int(binary.BigEndian.Uint32(buf[1:5]))
		if n <= 0 || n > 1<<22 || len(buf) < 5+n {
			break
		}
		out = append(out, string(buf[5:5+n]))
		buf = buf[5+n:]
	}
	return out
}

func findJSONError(v any) string {
	typed, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	return formatCursorError(typed["error"])
}

func formatCursorError(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		code, _ := typed["code"].(string)
		msg, _ := typed["message"].(string)
		// Connect carries the actionable provider message in ErrorDetails.debug.
		// The outer code can be resource_exhausted even for an incompatible client.
		if details, ok := typed["details"].([]any); ok {
			for _, item := range details {
				entry, _ := item.(map[string]any)
				debug, _ := entry["debug"].(map[string]any)
				info, _ := debug["details"].(map[string]any)
				title, _ := info["title"].(string)
				detail, _ := info["detail"].(string)
				if strings.TrimSpace(detail) != "" {
					return strings.TrimSpace(strings.Trim(title+": "+detail, ": "))
				}
			}
		}
		return strings.TrimSpace(strings.Trim(code+": "+msg, ": "))
	default:
		return ""
	}
}

func maybeGunzip(raw []byte) []byte {
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		return raw
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return raw
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 4<<20))
	if err != nil {
		return raw
	}
	return out
}

func clipChatBody(raw []byte) string {
	s := strings.Join(strings.Fields(strings.TrimSpace(string(raw))), " ")
	if s == "" {
		return "empty body"
	}
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

func connectFrame(msg []byte) []byte {
	out := make([]byte, 5+len(msg))
	binary.BigEndian.PutUint32(out[1:5], uint32(len(msg)))
	copy(out[5:], msg)
	return out
}

func pbString(field int, s string) []byte { return pbBytes(field, []byte(s)) }

func pbBytes(field int, v []byte) []byte {
	key := uint64(field<<3 | 2)
	out := uvarint(key)
	out = append(out, uvarint(uint64(len(v)))...)
	return append(out, v...)
}

func pbVarint(field int, v uint64) []byte {
	key := uint64(field << 3)
	return append(uvarint(key), uvarint(v)...)
}

func uvarint(v uint64) []byte {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], v)
	return buf[:n]
}

func newCursorID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[0:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" + hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:16])
}
