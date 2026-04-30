package skills

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParseSkillBytes(b []byte) (fm map[string]any, body []byte, err error) {
	fm = map[string]any{}
	b = stripUTF8BOM(b)
	if !bytes.HasPrefix(b, []byte("---")) {
		return fm, b, nil
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
	yamlBytes := bytes.Join(yamlLines, []byte("\n"))
	yamlBytes = bytes.TrimSpace(yamlBytes)
	if bodyStart >= 0 {
		body = bytes.Join(lines[bodyStart:], []byte("\n"))
		if len(yamlBytes) > 0 {
			if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
				return nil, nil, err
			}
		}
	} else {
		if len(yamlBytes) > 0 {
			if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
				return fm, bytes.TrimSpace(rest), nil
			}
			body = nil
		} else {
			body = bytes.TrimSpace(rest)
		}
	}
	if fm == nil {
		fm = map[string]any{}
	}
	return fm, body, nil
}

func stripUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
		return b[3:]
	}
	return b
}

func WriteSkillMarkdown(path string, fm map[string]any, body []byte) error {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	if len(fm) > 0 {
		enc, err := yaml.Marshal(fm)
		if err != nil {
			return err
		}
		buf.Write(enc)
	}
	buf.WriteString("---\n")
	if len(body) > 0 {
		body = bytes.TrimPrefix(body, []byte("\n"))
		buf.Write(body)
	}
	tmp := path + fmt.Sprintf(".tmp.%d", os.Getpid())
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ParseSkillFrontMatter(mdPath string) (map[string]any, error) {
	b, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}
	fm, _, err := ParseSkillBytes(b)
	return fm, err
}

func DisplayNameFromFrontMatter(fm map[string]any, fallback string) string {
	if fm != nil {
		for _, k := range []string{"name", "title"} {
			if v, ok := fm[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return fallback
}

func DescriptionFromFrontMatter(fm map[string]any) string {
	if fm == nil {
		return ""
	}
	for _, k := range []string{"description", "summary", "desc"} {
		if v, ok := fm[k]; ok && v != nil {
			if s, ok := v.(string); ok {
				if t := strings.TrimSpace(s); t != "" {
					return t
				}
				continue
			}
			if t := strings.TrimSpace(fmt.Sprint(v)); t != "" {
				return t
			}
		}
	}
	return ""
}

func SkillMarkdownBody(mdPath string) (string, error) {
	b, err := os.ReadFile(mdPath)
	if err != nil {
		return "", err
	}
	_, body, err := ParseSkillBytes(b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func SkillUserMessagePayload(entry SkillEntry) (string, error) {
	p := strings.TrimSpace(entry.SkillMdPath)
	if p == "" {
		return "", fmt.Errorf("skill %q has no SKILL.md path", strings.TrimSpace(entry.Name))
	}
	body, err := SkillMarkdownBody(p)
	if err != nil {
		return "", fmt.Errorf("read skill %q: %w", strings.TrimSpace(entry.Name), err)
	}
	if body == "" {
		body = "(empty skill body)"
	}
	return fmt.Sprintf("Apply the agent skill %q:\n\n%s", strings.TrimSpace(entry.Name), body), nil
}

func SkillInputPrefillText(entry SkillEntry) (string, error) {
	p := strings.TrimSpace(entry.SkillMdPath)
	if p == "" {
		return "", fmt.Errorf("skill %q has no SKILL.md path", strings.TrimSpace(entry.Name))
	}
	body, err := SkillMarkdownBody(p)
	if err != nil {
		return "", fmt.Errorf("read skill %q: %w", strings.TrimSpace(entry.Name), err)
	}
	if body == "" {
		return "(empty skill body)", nil
	}
	return body, nil
}
