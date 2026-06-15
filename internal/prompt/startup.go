package prompt

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

var isInteractiveSession = stdinIsTerminal

func SetInteractiveSessionCheckForTest(fn func() bool) func() {
	prev := isInteractiveSession
	isInteractiveSession = fn
	return func() { isInteractiveSession = prev }
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func expectedSHA(name string, saved map[string]string) (string, bool) {
	if saved == nil {
		return "", false
	}
	sha, ok := saved[name]
	return sha, ok && strings.TrimSpace(sha) != ""
}

func findTamperedTemplates(saved map[string]string) ([]string, error) {
	var tampered []string
	for _, name := range TemplateNames() {
		exp, ok := expectedSHA(name, saved)
		if !ok {
			continue
		}
		content, err := ReadTemplateFile(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if SHA256Hex(content) != exp {
			tampered = append(tampered, name)
		}
	}
	sort.Strings(tampered)
	return tampered, nil
}

func StartupTemplates(cfg *config.Root, out io.Writer, readLine func(string) (string, error)) error {
	if err := EnsureTemplatesInstalled(); err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	if cfg.PromptTemplates == nil {
		cfg.PromptTemplates = map[string]string{}
	}
	tampered, err := findTamperedTemplates(cfg.PromptTemplates)
	if err != nil {
		return err
	}
	if len(tampered) == 0 {
		return nil
	}
	for _, name := range tampered {
		fmt.Fprintf(out, "%s template has been modified\n", name)
	}
	if err := resolveTemplatePrompts(cfg, tampered, out, readLine); err != nil {
		return err
	}
	return config.Save(cfg)
}

func resolveTemplatePrompts(cfg *config.Root, names []string, out io.Writer, readLine func(string) (string, error)) error {
	if len(names) == 0 {
		return nil
	}
	if !isInteractiveSession() || readLine == nil {
		return nonInteractiveModifiedError(names)
	}
	mode := ""
	for i := 0; i < len(names); i++ {
		name := names[i]
		if mode == "accept" {
			acceptTemplateChange(cfg, name)
			continue
		}
		if mode == "deny" {
			if err := denyTemplateChange(cfg, name); err != nil {
				return err
			}
			continue
		}
		prompt := fmt.Sprintf("accept modifications for %s? denied request will reset template to default [yes(y) no(n) acceptAll(a) denyAll(d)]: ", name)
		line, err := readLine(prompt)
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			acceptTemplateChange(cfg, name)
		case "n", "no":
			if err := denyTemplateChange(cfg, name); err != nil {
				return err
			}
		case "a", "acceptall":
			mode = "accept"
			acceptTemplateChange(cfg, name)
		case "d", "denyall":
			mode = "deny"
			if err := denyTemplateChange(cfg, name); err != nil {
				return err
			}
		default:
			if err := denyTemplateChange(cfg, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func nonInteractiveModifiedError(modified []string) error {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return fmt.Errorf("modified prompt templates: %s (config path: %v)", strings.Join(modified, ", "), err)
	}
	tplDir, err := TemplatesDir()
	if err != nil {
		return fmt.Errorf("modified prompt templates: %s\nstart solomon in an interactive terminal to accept or deny changes, or align [prompt_templates] SHA256 hashes in %s with the template files", strings.Join(modified, ", "), cfgPath)
	}
	return fmt.Errorf("modified prompt templates: %s\nstart solomon in an interactive terminal to accept or deny changes, or align [prompt_templates] SHA256 hashes in %s with the files in %s", strings.Join(modified, ", "), cfgPath, tplDir)
}

func acceptTemplateChange(cfg *config.Root, name string) {
	content, err := ReadTemplateFile(name)
	if err != nil {
		return
	}
	if cfg.PromptTemplates == nil {
		cfg.PromptTemplates = map[string]string{}
	}
	cfg.PromptTemplates[name] = SHA256Hex(content)
}

func denyTemplateChange(cfg *config.Root, name string) error {
	if err := ResetTemplateToEmbedded(name); err != nil {
		return err
	}
	delete(cfg.PromptTemplates, name)
	return nil
}
