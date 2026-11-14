package server

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func importProjectEntry(root string, request projectFileOperation) error {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	destination := filepath.Clean(request.Destination)
	if strings.TrimSpace(request.Destination) == "" || filepath.IsAbs(destination) || destination == "." || destination == ".." || strings.HasPrefix(destination, ".."+string(filepath.Separator)) {
		return errors.New("invalid destination")
	}
	parent, err := safeWorkspacePath(root, filepath.Dir(destination))
	if err != nil {
		return err
	}
	target := filepath.Join(parent, filepath.Base(destination))
	if request.Content != nil {
		content, err := base64.StdEncoding.DecodeString(*request.Content)
		if err != nil {
			return errors.New("invalid file content")
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		_, writeErr := output.Write(content)
		closeErr := output.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(target)
		}
		return errors.Join(writeErr, closeErr)
	}
	if !filepath.IsAbs(request.Path) {
		return errors.New("invalid external source")
	}
	source, err := filepath.EvalSymlinks(request.Path)
	if err != nil {
		return err
	}
	if target == source || strings.HasPrefix(target, source+string(filepath.Separator)) {
		return errors.New("destination is inside source")
	}
	return copyProjectEntry(request.Path, target)
}
