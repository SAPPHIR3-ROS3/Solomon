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

func findTamperedTemplates(cfg *config.Root) ([]string, bool, error) {
	if cfg == nil {
		return nil, false, nil
	}
	if cfg.PromptTemplateModTime == nil {
		cfg.PromptTemplateModTime = map[string]int64{}
	}
	var tampered []string
	backfilled := false
	for _, name := range TemplateNames() {
		exp, ok := expectedSHA(name, cfg.PromptTemplates)
		if !ok {
			continue
		}
		modUnix, err := templateFileModTime(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, err
		}
		if savedMod, ok := cfg.PromptTemplateModTime[name]; ok && savedMod == modUnix {
			continue
		}
		content, err := ReadTemplateFile(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, err
		}
		if SHA256Hex(content) != exp {
			tampered = append(tampered, name)
			continue
		}
		cfg.PromptTemplateModTime[name] = modUnix
		backfilled = true
	}
	sort.Strings(tampered)
	return tampered, backfilled, nil
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
	configChanged := PurgeRetiredTemplateConfig(cfg)
	tampered, backfilled, err := findTamperedTemplates(cfg)
	if err != nil {
		return err
	}
	if len(tampered) == 0 {
		if configChanged || backfilled {
			return config.Save(cfg)
		}
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

func PurgeRetiredTemplateConfig(cfg *config.Root) bool {
	if cfg == nil || cfg.PromptTemplates == nil {
		return false
	}
	changed := false
	for _, name := range RetiredTemplateNames {
		if _, ok := cfg.PromptTemplates[name]; ok {
			delete(cfg.PromptTemplates, name)
			changed = true
		}
		if cfg.PromptTemplateModTime != nil {
			if _, ok := cfg.PromptTemplateModTime[name]; ok {
				delete(cfg.PromptTemplateModTime, name)
				changed = true
			}
		}
	}
	return changed
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
			if err := acceptTemplateChange(cfg, name); err != nil {
				return err
			}
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
			if err := acceptTemplateChange(cfg, name); err != nil {
				return err
			}
		case "n", "no":
			if err := denyTemplateChange(cfg, name); err != nil {
				return err
			}
		case "a", "acceptall":
			mode = "accept"
			if err := acceptTemplateChange(cfg, name); err != nil {
				return err
			}
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

func acceptTemplateChange(cfg *config.Root, name string) error {
	return recordTemplateAccepted(cfg, name)
}

func denyTemplateChange(cfg *config.Root, name string) error {
	if err := ResetTemplateToEmbedded(name); err != nil {
		return err
	}
	clearTemplateTracking(cfg, name)
	return nil
}
