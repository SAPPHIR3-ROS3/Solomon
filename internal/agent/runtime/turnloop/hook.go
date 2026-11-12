package turnloop

import (
	"io"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

const maxToolExecutionsPerRun = 256
const maxIdenticalToolRounds = 6

var toolExecutionLimit = maxToolExecutionsPerRun

func SetToolExecutionLimitForTest(limit int) func() {
	previous := toolExecutionLimit
	toolExecutionLimit = limit
	return func() { toolExecutionLimit = previous }
}

type toolBudget struct {
	executions int
	identical  int
	last       string
}

func (b *toolBudget) note(invs []tooling.Invocation) {
	b.executions += len(invs)
	var signature strings.Builder
	for _, inv := range invs {
		signature.WriteString(inv.Name)
		signature.WriteByte(0)
		signature.Write(inv.Args)
		signature.WriteByte(0)
	}
	if signature.String() == b.last {
		b.identical++
		return
	}
	b.last = signature.String()
	b.identical = 1
}

func (b toolBudget) exhausted() bool {
	return b.executions >= toolExecutionLimit || (b.identical >= maxIdenticalToolRounds && b.executions > 0)
}

var (
	generationStopMu sync.Mutex
	generationStop   func(error)

	streamGuardMu    sync.Mutex
	streamingDepth   int
	pendingSystemOut []string
)

func setGenerationStop(fn func(error)) {
	generationStopMu.Lock()
	generationStop = fn
	generationStopMu.Unlock()
}

func clearGenerationStop() {
	generationStopMu.Lock()
	generationStop = nil
	generationStopMu.Unlock()
}

func StopForTest(stopErr error) {
	generationStopMu.Lock()
	fn := generationStop
	generationStopMu.Unlock()
	if fn != nil {
		fn(stopErr)
	}
}

func EnterStreaming() {
	streamGuardMu.Lock()
	streamingDepth++
	streamGuardMu.Unlock()
}

func LeaveStreaming(w io.Writer) {
	streamGuardMu.Lock()
	if streamingDepth > 0 {
		streamingDepth--
	}
	depth := streamingDepth
	var queued []string
	if depth == 0 && len(pendingSystemOut) > 0 {
		queued = append([]string(nil), pendingSystemOut...)
		pendingSystemOut = nil
	}
	streamGuardMu.Unlock()
	for _, msg := range queued {
		termcolor.WriteSystem(w, msg)
	}
}

func writeSystemImmediate(w io.Writer, message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	_, _ = io.WriteString(w, "\n")
	termcolor.WriteSystem(w, message)
}

func WriteSystemDeferred(w io.Writer, message string) {
	message = termcolor.SystemMessageText(message)
	if message == "" {
		return
	}
	streamGuardMu.Lock()
	if streamingDepth > 0 {
		pendingSystemOut = append(pendingSystemOut, message)
		streamGuardMu.Unlock()
		return
	}
	streamGuardMu.Unlock()
	writeSystemImmediate(w, message)
}

func FlushPendingSystem(w io.Writer) {
	streamGuardMu.Lock()
	queued := append([]string(nil), pendingSystemOut...)
	pendingSystemOut = nil
	streamingDepth = 0
	streamGuardMu.Unlock()
	for _, msg := range queued {
		termcolor.WriteSystem(w, msg)
	}
}