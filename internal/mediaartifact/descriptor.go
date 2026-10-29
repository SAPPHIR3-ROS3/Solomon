// Package mediaartifact recognizes local visual artifacts and snapshots their
// static previews. It never downloads URLs or launches external media decoders.
package mediaartifact

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

type Descriptor struct {
	Kind        string `json:"type"`
	Path        string `json:"path"`
	PreviewPath string `json:"preview_path,omitempty"`
}

func KindForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".gif":
		return "gif"
	case ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff", ".svg", ".avif":
		return "image"
	case ".mp4", ".webm", ".mov", ".mkv", ".avi", ".m4v":
		return "video"
	}
	return ""
}

func LocalPath(raw, root string) string {
	if raw == "" || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "#") {
		return ""
	}
	drive := len(raw) > 2 && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/')
	if !drive {
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		if u.Scheme != "" {
			if u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") {
				return ""
			}
			raw = u.Path
			if filepath.Separator == '\\' && len(raw) > 2 && raw[0] == '/' && raw[2] == ':' {
				raw = raw[1:]
			}
		}
	}
	if filepath.Separator == '\\' && strings.HasPrefix(raw, "/") {
		volume := filepath.VolumeName(root)
		if volume == "" {
			cwd, _ := filepath.Abs(".")
			volume = filepath.VolumeName(cwd)
		}
		raw = volume + raw
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(root, raw)
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return ""
	}
	return abs
}

// Find accepts explicit media objects, local file resource links, and known
// media filenames returned by tools. JSON text blocks are decoded, never prose.
func Find(result any, root string) []Descriptor {
	var out []Descriptor
	seen := map[string]bool{}
	str := func(m map[string]any, keys ...string) string {
		for _, key := range keys {
			if s, ok := m[key].(string); ok && s != "" {
				return s
			}
		}
		return ""
	}
	var visit func(any, int)
	visit = func(v any, depth int) {
		if depth > 6 || len(out) >= 8 {
			return
		}
		switch v := v.(type) {
		case json.RawMessage:
			var decoded any
			if json.Unmarshal(v, &decoded) == nil {
				visit(decoded, depth+1)
			}
		case string:
			if len(v) < 1<<20 && json.Valid([]byte(v)) {
				var decoded any
				if json.Unmarshal([]byte(v), &decoded) == nil {
					visit(decoded, depth+1)
				}
			}
		case []any:
			for _, item := range v {
				visit(item, depth+1)
			}
		case map[string]any:
			kind := str(v, "type")
			path := str(v, "path", "file_path", "filePath", "output_path")
			if kind == "resource_link" {
				path = str(v, "uri")
				kind = ""
			}
			if kind != "image" && kind != "gif" && kind != "video" {
				kind = ""
			}
			if kind == "" {
				mime := str(v, "mime_type", "mimeType")
				if strings.HasPrefix(mime, "image/") {
					kind = "image"
				} else if strings.HasPrefix(mime, "video/") {
					kind = "video"
				} else {
					kind = KindForPath(path)
				}
			}
			if path = LocalPath(path, root); path != "" && kind != "" && !seen[path] {
				seen[path] = true
				preview := LocalPath(str(v, "preview_path", "previewPath", "thumbnail_path", "thumbnailPath", "poster_path", "posterPath"), root)
				if preview == "" && kind != "video" {
					preview = path
				}
				out = append(out, Descriptor{kind, path, preview})
			}
			for _, key := range []string{"artifacts", "media", "structuredContent", "content", "result", "results", "data", "output"} {
				if child, ok := v[key]; ok {
					visit(child, depth+1)
				}
			}
		}
	}
	visit(result, 0)
	return out
}

var markdownLink = regexp.MustCompile(`!?\[[^\]\n]*\]\((<[^>\n]+>|[^)\n]+)\)`)

func References(content, root string) []Descriptor {
	var out []Descriptor
	seen := map[string]bool{}
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			marker := trim[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		for _, match := range markdownLink.FindAllStringSubmatch(line, -1) {
			if len(out) >= 8 {
				return out
			}
			raw := strings.Trim(match[1], "<>")
			if unescaped, err := url.PathUnescape(raw); err == nil {
				raw = unescaped
			}
			path := LocalPath(raw, root)
			kind := KindForPath(path)
			if path == "" || kind == "" || seen[path] {
				continue
			}
			seen[path] = true
			preview := path
			if kind == "video" {
				preview = ""
			}
			out = append(out, Descriptor{kind, path, preview})
		}
	}
	return out
}
