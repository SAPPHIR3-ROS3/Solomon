package plan

import (
	"bytes"
	"time"

	"gopkg.in/yaml.v3"
)

type Meta struct {
	CreatedAt    string `yaml:"created_at"`
	CommitHash   string `yaml:"commit_hash"`
	LastCommitAt string `yaml:"last_commit_at"`
	Status       string `yaml:"status"`
}

func ParseDocument(b []byte) (Meta, []byte, error) {
	fm := map[string]any{}
	body := b
	if !bytes.HasPrefix(b, []byte("---")) {
		return metaFromMap(fm), body, nil
	}
	rest := b[3:]
	rest = bytes.TrimPrefix(rest, []byte("\r\n"))
	rest = bytes.TrimPrefix(rest, []byte("\n"))
	rest = bytes.ReplaceAll(rest, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(rest, []byte("\n"))
	var yamlLines [][]byte
	bodyStart := -1
	for i, line := range lines {
		if bytes.Equal(bytes.TrimSpace(line), []byte("---")) {
			bodyStart = i + 1
			break
		}
		yamlLines = append(yamlLines, line)
	}
	yamlBytes := bytes.TrimSpace(bytes.Join(yamlLines, []byte("\n")))
	if bodyStart >= 0 {
		body = bytes.Join(lines[bodyStart:], []byte("\n"))
		if len(yamlBytes) > 0 {
			if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
				return Meta{}, nil, err
			}
		}
	}
	return metaFromMap(fm), body, nil
}

func metaFromMap(fm map[string]any) Meta {
	m := Meta{Status: StatusNotBuilt}
	if fm == nil {
		return m
	}
	if v, ok := fm["created_at"].(string); ok {
		m.CreatedAt = v
	}
	if v, ok := fm["commit_hash"].(string); ok {
		m.CommitHash = v
	}
	if v, ok := fm["last_commit_at"].(string); ok {
		m.LastCommitAt = v
	}
	if v, ok := fm["status"].(string); ok && v != "" {
		m.Status = v
	}
	return m
}

func WriteDocument(meta Meta, body []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	fm := map[string]any{
		"created_at":     meta.CreatedAt,
		"commit_hash":    meta.CommitHash,
		"last_commit_at": meta.LastCommitAt,
		"status":         meta.Status,
	}
	enc, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}
	buf.Write(enc)
	buf.WriteString("---\n")
	if len(body) > 0 {
		body = bytes.TrimPrefix(body, []byte("\n"))
		buf.Write(body)
	}
	return buf.Bytes(), nil
}

func NewMeta(git GitMeta, status string) Meta {
	if status == "" {
		status = StatusNotBuilt
	}
	return Meta{
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		CommitHash:   git.CommitHash,
		LastCommitAt: git.LastCommitAt,
		Status:       status,
	}
}
