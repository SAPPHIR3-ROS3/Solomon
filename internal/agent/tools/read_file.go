package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mediaartifact"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
	_ "golang.org/x/image/webp"
)

func signatureReadFile(path, intent string) {}

const readFileDescription = "Read text or a PNG, JPEG, WebP, or GIF image relative to project root (absolute paths also accepted). GIFs attach only their first frame. Images are attached automatically for vision-capable models, including through orchestrate; no need to print image data. Optional startLine/endLine (1-based, inclusive) apply only to text. Discover unknown paths with find/listDir before reading; do not guess repository filenames. AGENTS.md instructions are loaded automatically when paths are activated; do not probe speculative AGENTS.md paths."

const maxReadImageBytes = 5 * 1024 * 1024

type readArgs struct {
	Path      string `json:"path"`
	StartLine *int   `json:"startLine,omitempty"`
	EndLine   *int   `json:"endLine,omitempty"`
}

func readFileOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("readFile", readFileDescription, map[string]any{
		"path": map[string]any{"type": "string", "description": "Path relative to project root, or an absolute path"},
		"startLine": map[string]any{
			"type":        "integer",
			"description": "Optional first line to read (1-based, inclusive)",
			"minimum":     1,
		},
		"endLine": map[string]any{
			"type":        "integer",
			"description": "Optional last line to read (1-based, inclusive)",
			"minimum":     1,
		},
	}, []string{"path"})
}

func appendReadFileDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureReadFile)
	if err != nil {
		return err
	}
	b.addBlock("readFile", readFileDescription, sig)
	return nil
}

func execReadFile(env *Env, raw json.RawMessage) (any, error) {
	var a readArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(env.ProjRoot, a.Path)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(p)
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w; discover the existing path with find or listDir before retrying", err)
		}
		return nil, err
	}
	defer f.Close()
	var header [12]byte
	n, err := f.ReadAt(header[:], 0)
	if err != nil && err != io.EOF {
		return nil, err
	}
	mime, recognized := images.MIMEForBinary(header[:n])
	isImage := recognized && (mime == "image/png" || mime == "image/jpeg" || mime == "image/webp" || mime == "image/gif")
	ext := strings.ToLower(filepath.Ext(p))
	imageExtension := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif"
	if imageExtension && !isImage {
		return nil, fmt.Errorf("readFile: invalid or unsupported image content in %s", p)
	}
	if isImage {
		if a.StartLine != nil || a.EndLine != nil {
			return nil, fmt.Errorf("readFile: startLine/endLine apply only to text, not images")
		}
		b, err := io.ReadAll(io.LimitReader(f, maxReadImageBytes+1))
		if err != nil {
			return nil, err
		}
		if len(b) > maxReadImageBytes {
			return nil, fmt.Errorf("readFile: image exceeds the 5 MiB limit")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("readFile: invalid image: %w", err)
		}
		attachment := images.Attachment{Path: p, MIMEType: mime, Data: b}
		if mime == "image/gif" {
			attachment, err = mediaartifact.Snapshot(mediaartifact.Descriptor{Kind: "gif", Path: p, PreviewPath: p})
			if err != nil {
				return nil, fmt.Errorf("readFile: invalid GIF: %w", err)
			}
			attachment.PresentationOnly = false
		}
		return map[string]any{
			"path": p, "type": "image", "mime_type": mime,
			"width": cfg.Width, "height": cfg.Height, "size_bytes": len(b),
			"content":                 fmt.Sprintf("Image attached: %s (%s, %dx%d). Requires a vision-capable model.", p, mime, cfg.Width, cfg.Height),
			images.ToolAttachmentsKey: []images.Attachment{attachment},
		}, nil
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	content := string(b)
	lines := splitFileLines(content)
	totalLines := len(lines)
	result := map[string]any{"path": p, "total_lines": totalLines}
	if a.StartLine == nil && a.EndLine == nil {
		result["content"] = content
		return result, nil
	}
	start := 1
	end := totalLines
	if a.StartLine != nil {
		start = *a.StartLine
	}
	if a.EndLine != nil {
		end = *a.EndLine
	}
	if start < 1 {
		return nil, fmt.Errorf("readFile: startLine must be >= 1")
	}
	if end < 1 {
		return nil, fmt.Errorf("readFile: endLine must be >= 1")
	}
	if end < start {
		return nil, fmt.Errorf("readFile: endLine must be >= startLine")
	}
	if totalLines == 0 {
		result["content"] = ""
		result["start_line"] = start
		result["end_line"] = end
		return result, nil
	}
	if start > totalLines {
		return nil, fmt.Errorf("readFile: startLine %d beyond file (%d lines)", start, totalLines)
	}
	if end > totalLines {
		end = totalLines
	}
	slice := lines[start-1 : end]
	result["content"] = strings.Join(slice, "\n")
	result["start_line"] = start
	result["end_line"] = end
	return result, nil
}

func splitFileLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func resolveProjectPath(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, filepath.Clean(p))
}
