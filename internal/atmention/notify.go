package atmention

import (
	"fmt"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type SkipReason string

const (
	SkipMissing    SkipReason = "missing"
	SkipGitignored SkipReason = "gitignored"
	SkipBinary     SkipReason = "binary"
	SkipExternal   SkipReason = "external"
	SkipDepth      SkipReason = "depth"
	SkipCycle      SkipReason = "cycle"
	SkipNotText    SkipReason = "not_text"
)

type SkipNotice struct {
	Reason SkipReason
	Tag    string
	Path   string
}

type Notifier struct {
	mu      sync.Mutex
	skips   []SkipNotice
	logged  map[string]struct{}
}

func NewNotifier() *Notifier {
	return &Notifier{logged: make(map[string]struct{})}
}

func (n *Notifier) Add(reason SkipReason, tag, path string) {
	if n == nil {
		return
	}
	tag = strings.TrimSpace(tag)
	path = strings.TrimSpace(path)
	n.mu.Lock()
	n.skips = append(n.skips, SkipNotice{Reason: reason, Tag: tag, Path: path})
	key := string(reason) + "\x00" + tag + "\x00" + path
	if _, ok := n.logged[key]; !ok {
		n.logged[key] = struct{}{}
		logging.Log(logging.WARNING_LOG_LEVEL, "@ include skipped", logging.LogOptions{Params: map[string]any{
			"reason": reason,
			"tag":    tag,
			"path":   path,
		}})
	}
	n.mu.Unlock()
}

func (n *Notifier) Merge(other *Notifier) {
	if n == nil || other == nil {
		return
	}
	other.mu.Lock()
	copySkips := append([]SkipNotice(nil), other.skips...)
	other.mu.Unlock()
	for _, s := range copySkips {
		n.Add(s.Reason, s.Tag, s.Path)
	}
}

func (n *Notifier) Messages() []string {
	if n == nil {
		return nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.skips) == 0 {
		return nil
	}
	out := make([]string, 0, len(n.skips))
	for _, s := range n.skips {
		out = append(out, formatSkipNotice(s))
	}
	return out
}

func formatSkipNotice(s SkipNotice) string {
	tag := s.Tag
	if tag != "" && !strings.HasPrefix(tag, "@") {
		tag = "@" + tag
	}
	switch s.Reason {
	case SkipMissing:
		return fmt.Sprintf("  %s: file not found (%s)", tag, s.Path)
	case SkipGitignored:
		return fmt.Sprintf("  %s: gitignored (%s) — read manually", tag, s.Path)
	case SkipBinary:
		return fmt.Sprintf("  %s: binary or too large (%s)", tag, s.Path)
	case SkipExternal:
		return fmt.Sprintf("  %s: outside project root (%s)", tag, s.Path)
	case SkipDepth:
		return fmt.Sprintf("  %s: include depth limit (%s)", tag, s.Path)
	case SkipCycle:
		return fmt.Sprintf("  %s: circular include (%s)", tag, s.Path)
	case SkipNotText:
		return fmt.Sprintf("  %s: not a text file (%s)", tag, s.Path)
	default:
		return fmt.Sprintf("  %s: skipped (%s)", tag, s.Path)
	}
}
