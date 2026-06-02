package llm

import (
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

type ProviderHTTPError = transport.ProviderHTTPError

func NewProviderHTTPError(status int, message string, retryAfter time.Duration) *ProviderHTTPError {
	return transport.NewProviderHTTPError(status, message, retryAfter)
}
