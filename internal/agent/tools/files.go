package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type fileOperationArgs struct {
	Path        string `json:"path"`
	Destination string `json:"destination"`
}

func execCreateDir(env *Env, raw json.RawMessage) (any, error) {
	var a fileOperationArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.Path == "" {
		return nil, fmt.Errorf("createDir: path is required")
	}
	path := resolveProjectPath(env.ProjRoot, a.Path)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(path)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "action": "directory_ready", "path": a.Path}, nil
}

func execStat(env *Env, raw json.RawMessage) (any, error) {
	var a fileOperationArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.Path == "" {
		return nil, fmt.Errorf("stat: path is required")
	}
	path := resolveProjectPath(env.ProjRoot, a.Path)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(path)
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return map[string]any{"path": a.Path, "exists": false}, nil
	}
	if err != nil {
		return nil, err
	}
	kind := "other"
	if info.Mode().IsRegular() {
		kind = "file"
	} else if info.IsDir() {
		kind = "directory"
	} else if info.Mode()&os.ModeSymlink != 0 {
		kind = "symlink"
	}
	return map[string]any{"path": a.Path, "exists": true, "type": kind, "size": info.Size(), "mode": uint32(info.Mode().Perm()), "modified": info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}, nil
}

func copyInvocationPaths(env *Env, raw json.RawMessage) []string {
	var a fileOperationArgs
	if env == nil || json.Unmarshal(raw, &a) != nil {
		return nil
	}
	return []string{resolveProjectPath(env.ProjRoot, a.Destination)}
}

func execCopyFile(env *Env, raw json.RawMessage) (any, error) {
	var a fileOperationArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if a.Path == "" || a.Destination == "" {
		return nil, fmt.Errorf("copyFile: path and destination are required")
	}
	source := resolveProjectPath(env.ProjRoot, a.Path)
	destination := resolveProjectPath(env.ProjRoot, a.Destination)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(source)
		env.ActivateInstructionsFromAbsPath(destination)
	}
	input, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("copyFile: source must be a regular file")
	}
	stageBeforeEdit(env, destination)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return nil, err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return nil, err
	}
	written, copyErr := io.Copy(output, input)
	if copyErr == nil {
		copyErr = output.Chmod(info.Mode().Perm())
	}
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(destination)
		if copyErr != nil {
			return nil, copyErr
		}
		return nil, closeErr
	}
	stageAfterEdit(env, "create", destination, "")
	stageProjectPath(env, destination)
	return map[string]any{"ok": true, "action": "copied", "from": a.Path, "to": a.Destination, "bytes": written}, nil
}
