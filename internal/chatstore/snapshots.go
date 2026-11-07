package chatstore

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"

func removeInternalSnapshots(session *Session) bool {
	changed := false
	for name := range session.FileChanges {
		if filechanges.IsInternalSnapshotPath(name) {
			delete(session.FileChanges, name)
			changed = true
		}
	}
	clean := func(messages []Message) {
		for i := range messages {
			original := messages[i].FileChanges
			first := -1
			for j, change := range original {
				if filechanges.IsInternalSnapshotPath(change.Path) {
					first = j
					break
				}
			}
			if first < 0 {
				continue
			}
			changed = true
			kept := append([]FileChangeSummary{}, original[:first]...)
			for _, change := range original[first+1:] {
				if !filechanges.IsInternalSnapshotPath(change.Path) {
					kept = append(kept, change)
				}
			}
			messages[i].FileChanges = kept
		}
	}
	clean(session.Messages)
	for i := range session.Branches {
		clean(session.Branches[i].Messages)
	}
	for i := range session.UncompactedRaw {
		clean(session.UncompactedRaw[i].Messages)
	}
	return changed
}
