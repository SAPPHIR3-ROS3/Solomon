package config

import (
	"fmt"
	"github.com/openai/openai-go/v2/shared"
	"strings"
)

func ParseReasoningEffortToken(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Join(strings.Fields(s), "-")
	switch s {
	case "none":
		return "none", nil
	case "minimal":
		return "minimal", nil
	case "low":
		return "low", nil
	case "med", "medium":
		return "medium", nil
	case "high":
		return "high", nil
	case "xhigh", "x-high", "extra-high":
		return "xhigh", nil
	case "max":
		return "max", nil
	default:
		return "", fmt.Errorf("reasoning must be none, minimal, low, med, medium, high, xhigh (extra high), or max")
	}
}

func (r *Root) ReasoningEffortIsNone() bool {
	if r == nil {
		return false
	}
	c, err := ParseReasoningEffortToken(r.ReasoningEffort)
	return err == nil && c == "none"
}

func (r *Root) GlobalReasoningEffort() shared.ReasoningEffort {
	if r == nil {
		return ""
	}
	c, err := ParseReasoningEffortToken(r.ReasoningEffort)
	if err != nil {
		return ""
	}
	switch c {
	case "none":
		return shared.ReasoningEffort("none")
	case "low":
		return shared.ReasoningEffortLow
	case "medium":
		return shared.ReasoningEffortMedium
	case "high":
		return shared.ReasoningEffortHigh
	case "minimal", "xhigh", "max":
		// The OpenAI SDK's named constants lag some provider-specific effort
		// values, but the wire type intentionally accepts arbitrary strings.
		return shared.ReasoningEffort(c)
	default:
		return ""
	}
}

func (r *Root) ReasoningEffortLabel() string {
	if r == nil {
		return ""
	}
	c, err := ParseReasoningEffortToken(r.ReasoningEffort)
	if err != nil {
		return ""
	}
	return c
}

func (r *Root) ReasoningEffortDisplayLabel() string {
	if lbl := r.ReasoningEffortLabel(); lbl != "" {
		return lbl
	}
	return "none"
}

func (r *Root) SubagentReasoningEffortIsNone() bool {
	if r == nil {
		return true
	}
	if strings.TrimSpace(r.SubagentReasoningEffort) == "" {
		return true
	}
	c, err := ParseReasoningEffortToken(r.SubagentReasoningEffort)
	return err == nil && c == "none"
}

func (r *Root) SubagentReasoningEffortLabel() string {
	if r == nil {
		return ""
	}
	c, err := ParseReasoningEffortToken(r.SubagentReasoningEffort)
	if err != nil {
		return ""
	}
	return c
}

func (r *Root) SubagentReasoningEffortDisplayLabel() string {
	if lbl := r.SubagentReasoningEffortLabel(); lbl != "" {
		return lbl
	}
	return "none"
}

func (r *Root) EffectiveSubagentReasoningEffort(override string) (canonical string, forceDisable bool) {
	if o := strings.TrimSpace(override); o != "" {
		c, err := ParseReasoningEffortToken(o)
		if err != nil {
			return "none", true
		}
		return c, c == "none"
	}
	if r != nil && strings.TrimSpace(r.SubagentReasoningEffort) != "" {
		c, err := ParseReasoningEffortToken(r.SubagentReasoningEffort)
		if err != nil {
			return "none", true
		}
		return c, c == "none"
	}
	return "none", true
}
