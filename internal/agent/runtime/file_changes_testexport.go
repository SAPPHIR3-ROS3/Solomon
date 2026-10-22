package agentruntime

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
)

func (r *Runtime) RecordChatFileChangesForTest(seq int, changes []filechanges.Change, commits []filechanges.Commit) {
	r.recordChatFileChanges(seq, changes, commits)
}
func (r *Runtime) PrintChatFileChangesForTest(seq int)      { r.printChatFileChanges(seq) }
func (r *Runtime) SaveTurnFileChangesForTest(seq int) error { return r.saveTurnFileChanges(seq) }
