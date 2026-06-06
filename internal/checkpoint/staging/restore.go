package staging

import (
	"fmt"
	"os"
	"path/filepath"
)

type RestoreResult struct {
	FilesWritten int
	FilesRemoved int
	Warnings     []string
}

func (s *Store) RestoreToCheckpoint(seq int, projRoot string) (RestoreResult, error) {
	var res RestoreResult
	if s == nil {
		return res, nil
	}
	projRoot, err := filepath.Abs(projRoot)
	if err != nil {
		return res, err
	}
	type fileState struct {
		exists  bool
		content []byte
	}
	states := make(map[string]fileState)
	for path, bl := range s.Baselines {
		if bl.Missing {
			states[path] = fileState{exists: false}
			continue
		}
		data, err := s.readBlob(bl.Blob)
		if err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("baseline %s: %v", path, err))
			continue
		}
		states[path] = fileState{exists: true, content: data}
	}
	aliases := make(map[string]string)
	for p := range states {
		aliases[p] = p
	}
	resolve := func(p string) string {
		for {
			next, ok := aliases[p]
			if !ok || next == p {
				return p
			}
			p = next
		}
	}
	for _, op := range sortedOps(s.Ops) {
		if op.CpSeq > seq {
			continue
		}
		switch op.Kind {
		case "delete":
			key := resolve(op.Path)
			states[key] = fileState{exists: false}
		case "rename":
			src := resolve(op.Path)
			dst := filepath.Clean(op.RenameTo)
			st, ok := states[src]
			if !ok {
				st = fileState{exists: false}
			}
			delete(states, src)
			states[dst] = st
			aliases[op.Path] = dst
			aliases[src] = dst
		case "create", "write", "patch":
			key := resolve(op.Path)
			data, err := s.readBlob(op.Blob)
			if err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("op %s cp%d: %v", op.Kind, op.CpSeq, err))
				continue
			}
			states[key] = fileState{exists: true, content: data}
		}
	}
	touched := make(map[string]struct{})
	for p := range s.Baselines {
		touched[resolve(p)] = struct{}{}
	}
	for _, op := range s.Ops {
		touched[resolve(op.Path)] = struct{}{}
		if op.RenameTo != "" {
			touched[filepath.Clean(op.RenameTo)] = struct{}{}
		}
	}
	for path := range touched {
		want, ok := states[path]
		if !ok {
			want = fileState{exists: false}
		}
		if !stringsHasPrefix(path, projRoot) {
			res.Warnings = append(res.Warnings, fmt.Sprintf("skip path outside project: %s", path))
			continue
		}
		if want.exists {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				res.Warnings = append(res.Warnings, err.Error())
				continue
			}
			if err := os.WriteFile(path, want.content, 0o600); err != nil {
				res.Warnings = append(res.Warnings, err.Error())
				continue
			}
			res.FilesWritten++
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				res.Warnings = append(res.Warnings, err.Error())
				continue
			}
			res.FilesRemoved++
		}
	}
	return res, nil
}

func stringsHasPrefix(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	sep := string(filepath.Separator)
	return len(path) > len(root) && path[:len(root)] == root && path[len(root)] == sep[0]
}
