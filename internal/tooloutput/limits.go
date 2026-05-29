package tooloutput

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

const (
	DefaultMaxBytes = 65536
	DefaultMaxLines = 2048
)

type Limits struct {
	MaxBytes int
	MaxLines int
}

func DefaultLimits() Limits {
	return Limits{
		MaxBytes: DefaultMaxBytes,
		MaxLines: DefaultMaxLines,
	}
}

func LimitsFromConfig(r *config.Root) Limits {
	lim := DefaultLimits()
	if r == nil {
		return lim
	}
	if r.ToolOutput.MaxBytes > 0 {
		lim.MaxBytes = r.ToolOutput.MaxBytes
	}
	if r.ToolOutput.MaxLines > 0 {
		lim.MaxLines = r.ToolOutput.MaxLines
	}
	return lim
}
