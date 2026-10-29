package server

import (
	"encoding/base64"
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var imageTagPattern = regexp.MustCompile(`\[img-(\d+)\]`)

func (a *chatAPI) handleChatImage(w http.ResponseWriter, _ *http.Request, projectID, chatID string, seq int) {
	var filePath string
	if sess, err := chatstore.ReadSession(projectID, chatID); err == nil {
		filePath = sess.ImageFiles[seq]
	} else if sub, subErr := chatstore.FindSubSessionByID(projectID, chatID); subErr == nil {
		filePath = sub.ImageFiles[seq]
	}
	if filePath == "" || !isSafeProjectImagePath(projectID, filePath) {
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
		return
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(b))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func isSafeProjectImagePath(projectID, filePath string) bool {
	root, err := paths.ChatImagesDir(projectID)
	if err != nil {
		return false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	candidate, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(base, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (a *chatAPI) attachIncomingImages(projectID string, sess *chatstore.Session, content string, incoming []incomingImage) (string, error) {
	if len(incoming) == 0 {
		return content, nil
	}
	tags := imageTagPattern.FindAllStringSubmatch(content, -1)
	created := make([]string, 0, len(incoming))
	cleanup := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}
	nextSeq := sess.ImageSeq
	nextContent := content
	addedFiles := make(map[int]string, len(incoming))
	for index, incomingImage := range incoming {
		dataURL := strings.TrimSpace(incomingImage.Data)
		if dataURL == "" {
			dataURL = strings.TrimSpace(incomingImage.URL)
		}
		data, err := decodeImageDataURL(dataURL)
		if err != nil {
			cleanup()
			return content, err
		}
		if _, ok := images.MIMEForBinary(data); !ok {
			cleanup()
			return content, errors.New("unsupported image attachment")
		}
		seq := nextSeq
		nextSeq++
		filePath, err := writeIncomingImage(projectID, sess.ID, seq, data)
		if err != nil {
			cleanup()
			return content, err
		}
		created = append(created, filePath)
		addedFiles[seq] = filePath
		if index < len(tags) {
			nextContent = strings.ReplaceAll(nextContent, tags[index][0], images.VisibleTag(seq))
		} else {
			nextContent = strings.TrimSpace(nextContent + " " + images.VisibleTag(seq))
		}
	}
	if sess.ImageFiles == nil {
		sess.ImageFiles = make(map[int]string, len(addedFiles))
	}
	for seq, filePath := range addedFiles {
		sess.ImageFiles[seq] = filePath
	}
	sess.ImageSeq = nextSeq
	return nextContent, nil
}

func writeIncomingImage(projectID, chatID string, seq int, data []byte) (string, error) {
	stamp := time.Now().UTC()
	for attempt := 0; attempt < 1000; attempt++ {
		filePath, err := paths.ImagePath(projectID, chatID, seq, stamp)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
			return "", err
		}
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			stamp = stamp.Add(time.Millisecond)
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			_ = os.Remove(filePath)
			return "", writeErr
		}
		if closeErr != nil {
			_ = os.Remove(filePath)
			return "", closeErr
		}
		return filePath, nil
	}
	return "", errors.New("unable to allocate a unique image path")
}

func cloneImageFiles(files map[int]string) map[int]string {
	if len(files) == 0 {
		return nil
	}
	clone := make(map[int]string, len(files))
	for seq, filePath := range files {
		clone[seq] = filePath
	}
	return clone
}

func restoreAttachedImages(sess *chatstore.Session, previous map[int]string, previousSeq int) {
	if sess == nil {
		return
	}
	for seq, filePath := range sess.ImageFiles {
		if previous[seq] != filePath {
			_ = os.Remove(filePath)
		}
	}
	sess.ImageFiles = cloneImageFiles(previous)
	sess.ImageSeq = previousSeq
}

func decodeImageDataURL(value string) ([]byte, error) {
	if !strings.HasPrefix(value, "data:") {
		return nil, errors.New("image attachment must be a data URL")
	}
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 || !strings.Contains(parts[0], ";base64") {
		return nil, errors.New("invalid image data URL")
	}
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid image data")
	}
	if len(data) == 0 {
		return nil, errors.New("empty image attachment")
	}
	return data, nil
}

func incomingClips(values []incomingClip) []atmention.TerminalClip {
	out := make([]atmention.TerminalClip, 0, len(values))
	for _, value := range values {
		out = append(out, atmention.TerminalClip{End: value.End, Start: value.Start, Tag: value.Tag, Text: value.Text})
	}
	return out
}
