package codex

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	codexchat "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
	"github.com/openai/openai-go/v2/option"
)

func WithChatGPTSubMiddleware(accountID string) option.RequestOption {
	return option.WithMiddleware(chatGPTSubMiddleware(accountID))
}

func chatGPTSubMiddleware(accountID string) option.Middleware {
	return func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		if req.Method != http.MethodPost || !strings.Contains(req.URL.Path, "chat/completions") {
			return next(req)
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		var chat map[string]any
		if err := json.Unmarshal(bodyBytes, &chat); err != nil {
			return nil, fmt.Errorf("codex proxy: parse chat body: %w", err)
		}
		clientStream, _ := chat["stream"].(bool)
		model, _ := chat["model"].(string)
		if !clientStream {
			chat["stream"] = true
		}
		codexBody, err := codexchat.ChatCompletionToCodexBody(chat)
		if err != nil {
			return nil, err
		}
		upReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, ChatGPTResponsesURL, bytes.NewReader(codexBody))
		if err != nil {
			return nil, err
		}
		applyCodexUpstreamHeaders(upReq, req.Header.Get("Authorization"), strings.TrimSpace(accountID))
		upResp, err := http.DefaultClient.Do(upReq)
		if err != nil {
			return nil, err
		}
		if upResp.StatusCode != http.StatusOK {
			body, _ := codexchat.DrainUpstreamError(upResp)
			return nil, codexchat.ChatGPTSubUpstreamError(upResp.StatusCode, body, model)
		}
		if clientStream {
			pr, pw := io.Pipe()
			go func() {
				defer upResp.Body.Close()
				err := codexchat.RewriteCodexSSEStream(upResp.Body, pw, model)
				_ = pw.CloseWithError(err)
			}()
			return codexProxyResponse(upResp, pr, "text/event-stream"), nil
		}
		defer upResp.Body.Close()
		jsonBody, err := codexchat.BufferChatCompletionFromCodexSSE(upResp.Body, model)
		if err != nil {
			return nil, err
		}
		return codexProxyResponse(upResp, io.NopCloser(bytes.NewReader(jsonBody)), "application/json"), nil
	}
}

func applyCodexUpstreamHeaders(req *http.Request, authorization, accountID string) {
	bearer := strings.TrimSpace(authorization)
	if len(bearer) >= 7 && strings.EqualFold(bearer[:7], "Bearer ") {
		bearer = strings.TrimSpace(bearer[7:])
	}
	req.Header.Set("authorization", "Bearer "+bearer)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")
	req.Header.Set("openai-beta", "responses=experimental")
	req.Header.Set("originator", Originator)
	req.Header.Set("user-agent", UserAgent)
	req.Header.Set("version", "0.132.0")
	req.Header.Set("session_id", randomHexID())
	if accountID != "" {
		req.Header.Set("chatgpt-account-id", accountID)
	}
	req.Header.Set("x-codex-beta-features", "multi_agent,apps,prevent_idle_sleep")
	req.Header.Set("x-codex-turn-metadata", fmt.Sprintf(`{"turn_id":%q,"sandbox":"none"}`, randomHexID()))
}

func randomHexID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func codexProxyResponse(up *http.Response, body io.ReadCloser, contentType string) *http.Response {
	out := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       body,
		Request:    up.Request,
	}
	out.Header.Set("Content-Type", contentType)
	return out
}
