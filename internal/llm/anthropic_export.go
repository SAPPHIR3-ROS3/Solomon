package llm

import (
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/anthropic"
)

const AnthropicAPIVersion = anthropic.APIVersion

const AnthropicOAuthBeta = anthropic.OAuthBeta

type AnthropicAuth = anthropic.Auth

type AnthropicBackend = anthropic.Backend

type AnthropicUsagePayload = anthropic.UsagePayload

func AnthropicAuthFromAPIKey(token string) AnthropicAuth {
	return anthropic.AuthFromAPIKey(token)
}

func AnthropicAuthFromOAuthBearer(token string) AnthropicAuth {
	return anthropic.AuthFromOAuthBearer(token)
}

func NewAnthropicBackend(baseURL string, auth AnthropicAuth) *AnthropicBackend {
	return anthropic.NewBackend(baseURL, auth)
}

func NewAnthropicBackendWithClient(baseURL string, auth AnthropicAuth, client *http.Client) *AnthropicBackend {
	return anthropic.NewBackendWithClient(baseURL, auth, client)
}

func AnthropicMessagesURL(base string) string {
	return anthropic.MessagesURL(base)
}

func NormalizeAnthropicUsage(u AnthropicUsagePayload) UsageStats {
	return anthropic.NormalizeUsage(u)
}
