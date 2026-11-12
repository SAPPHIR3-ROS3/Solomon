package webfetch

import (
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func logFetchAttempt(host string, attempt int, budget, duration time.Duration, err error) {
	params := map[string]any{
		"host": host, "attempt": attempt, "max_attempts": maxFetchRetries,
		"budget_ms": budget.Milliseconds(), "duration_ms": duration.Milliseconds(),
	}
	level := logging.DEBUG_LOG_LEVEL
	if err != nil {
		kind, retryable := classifyFetchFailure(err)
		params["error_kind"], params["retryable"] = kind, retryable
		level = logging.WARNING_LOG_LEVEL
	}
	logging.Log(level, "HTTP fetch attempt completed", logging.LogOptions{Params: params})
}
