package chat

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

type UpstreamError struct {
	StatusCode int
	Message    string
	Payload    json.RawMessage
}

func (e *UpstreamError) Error() string {
	return "ChatGPT Sub: " + e.Message
}

func (e *UpstreamError) Unwrap() error {
	return transport.NewProviderHTTPError(e.StatusCode, e.Message, 0)
}

func (e *UpstreamError) DiagnosticFields() map[string]any {
	var payload upstreamErrorPayload
	if json.Unmarshal(e.Payload, &payload) != nil {
		return nil
	}
	for payload.Error != nil {
		payload = *payload.Error
	}
	return map[string]any{"provider_error_type": payload.Type, "provider_error_code": payload.Code}
}
