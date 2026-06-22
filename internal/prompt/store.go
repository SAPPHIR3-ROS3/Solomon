package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

var RetiredTemplateNames = []string{"plan", "build"}

func RemoveRetiredTemplates() error {
	for _, name := range RetiredTemplateNames {
		p, err := templateFilePath(name)
		if err != nil {
			return err
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func TemplatesDir() (string, error) {
	return paths.PromptTemplatesDir()
}

func templateFilePath(name string) (string, error) {
	d, err := TemplatesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name+".tmpl"), nil
}

func SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func ReadTemplateFile(name string) (string, error) {
	p, err := templateFilePath(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func WriteTemplateFile(name, content string) error {
	p, err := templateFilePath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func EnsureTemplatesInstalled() error {
	if err := EnsureTemplatesInstalledOnlyDir(); err != nil {
		return err
	}
	if err := RemoveRetiredTemplates(); err != nil {
		return err
	}
	names := TemplateNames()
	sort.Strings(names)
	for _, name := range names {
		p, err := templateFilePath(name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(p); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := writeEmbeddedTemplate(name); err != nil {
			return err
		}
	}
	return nil
}

func EnsureTemplatesInstalledOnlyDir() error {
	return paths.EnsurePromptTemplatesDir()
}

func TemplateContent(name string) (string, error) {
	if s, err := ReadTemplateFile(name); err == nil {
		return s, nil
	}
	emb, ok := EmbeddedTemplate(name)
	if !ok {
		return "", os.ErrNotExist
	}
	return emb, nil
}

func templateFileModTime(name string) (int64, error) {
	p, err := templateFilePath(name)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
}

func recordTemplateAccepted(cfg *config.Root, name string) error {
	content, err := ReadTemplateFile(name)
	if err != nil {
		return err
	}
	if cfg.PromptTemplates == nil {
		cfg.PromptTemplates = map[string]string{}
	}
	if cfg.PromptTemplateModTime == nil {
		cfg.PromptTemplateModTime = map[string]int64{}
	}
	cfg.PromptTemplates[name] = SHA256Hex(content)
	mod, err := templateFileModTime(name)
	if err != nil {
		return err
	}
	cfg.PromptTemplateModTime[name] = mod
	return nil
}

func clearTemplateTracking(cfg *config.Root, name string) {
	if cfg.PromptTemplates != nil {
		delete(cfg.PromptTemplates, name)
	}
	if cfg.PromptTemplateModTime != nil {
		delete(cfg.PromptTemplateModTime, name)
	}
}

func ResetTemplateToEmbedded(name string) error {
	emb, ok := EmbeddedTemplate(name)
	if !ok {
		return os.ErrNotExist
	}
	return WriteTemplateFile(name, emb)
}
