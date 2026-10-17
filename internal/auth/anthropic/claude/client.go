package claude

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const APIVersion = "2023-06-01"
const OAuthBeta = "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"

// NewClient configures the official SDK for inference and model discovery.
// OAuth credentials remain managed by Solomon.
func NewClient(base, token string, oauth bool, client *http.Client) sdk.Client {
	if strings.TrimSpace(base) == "" {
		base = "https://api.anthropic.com"
	}
	// Users can configure a root, a versioned API base, or a full endpoint,
	// including a reverse-proxy prefix and query parameters.
	u, err := url.Parse(strings.TrimSpace(base))
	opts := []option.RequestOption{option.WithoutEnvironmentDefaults(), option.WithMaxRetries(0)}
	if err == nil {
		for key, values := range u.Query() {
			for _, value := range values {
				opts = append(opts, option.WithQueryAdd(key, value))
			}
		}
		u.RawQuery, u.Fragment = "", ""
		u.Path = strings.TrimRight(u.Path, "/")
		for _, suffix := range []string{"/v1/messages", "/v1/models", "/v1"} {
			if strings.HasSuffix(u.Path, suffix) {
				u.Path = strings.TrimSuffix(u.Path, suffix)
				break
			}
		}
		u.RawPath = ""
		u.Path += "/"
		base = u.String()
	}
	opts = append(opts, option.WithBaseURL(base), option.WithHeader("anthropic-version", APIVersion))
	if client != nil {
		opts = append(opts, option.WithHTTPClient(client))
	}
	if oauth {
		opts = append(opts, option.WithAuthToken(token), option.WithHeaderDel("x-api-key"),
			option.WithHeader("anthropic-beta", OAuthBeta),
			option.WithHeader("user-agent", "claude-cli/"+Version()),
			option.WithHeader("x-app", "cli"),
			option.WithHeader("anthropic-dangerous-direct-browser-access", "true"))
	} else {
		opts = append(opts, option.WithAPIKey(token), option.WithHeaderDel("Authorization"))
	}
	return sdk.NewClient(opts...)
}

// SDKError preserves Solomon's retry policy and Retry-After handling. The SDK's own retries
// are disabled so a single turn never acquires two independent retry loops.
func SDKError(err error) error {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		// SSE error events arrive after HTTP 200. Classify their actual error
		// type so the existing retry policy also handles mid-stream overloads.
		if status < 400 {
			switch string(apiErr.Type()) {
			case "overloaded_error":
				status = http.StatusServiceUnavailable
			case "rate_limit_error":
				status = http.StatusTooManyRequests
			case "api_error":
				status = http.StatusInternalServerError
			case "authentication_error":
				status = http.StatusUnauthorized
			case "permission_error":
				status = http.StatusForbidden
			case "invalid_request_error":
				status = http.StatusBadRequest
			}
		}
		return transport.NewProviderHTTPError(status, apiErr.RawJSON(), transport.ParseRetryAfterHeader(apiErr.Response))
	}
	return err
}
