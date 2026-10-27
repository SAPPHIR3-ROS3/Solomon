package chatstore

import (
	"fmt"
	"strings"
	"time"
)

// Keep the original user message so expanded mentions, image references and
// checkpoint metadata remain identical on the new attempt.
func (s *Session) TruncateForRegeneration() error {
	if s != nil {
		for i := len(s.Messages) - 1; i >= 0; i-- {
			m := s.Messages[i]
			if m.Role != "user" || m.GUIHidden || strings.HasPrefix(strings.TrimSpace(m.Content), "tool_result(") {
				continue
			}
			s.Messages = s.Messages[:i+1]
			s.LastMessageAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("no user message to regenerate")
}
