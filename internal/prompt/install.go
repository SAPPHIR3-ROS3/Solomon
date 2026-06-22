package prompt

import (
	"io"
	"os"
	"sort"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func InstallTemplates(cfg *config.Root, out io.Writer, readLine func(string) (string, error)) error {
	if err := EnsureTemplatesInstalledOnlyDir(); err != nil {
		return err
	}
	if err := RemoveRetiredTemplates(); err != nil {
		return err
	}
	if cfg == nil {
		cfg = config.EmptyRoot()
	}
	if cfg.PromptTemplates == nil {
		cfg.PromptTemplates = map[string]string{}
	}
	purged := PurgeRetiredTemplateConfig(cfg)
	toCreate, toUpgrade, toPrompt, err := planTemplateInstall(cfg.PromptTemplates)
	if err != nil {
		return err
	}
	if len(toCreate)+len(toUpgrade)+len(toPrompt) == 0 {
		if purged {
			return config.Save(cfg)
		}
		return nil
	}
	if len(toPrompt) > 0 {
		if err := resolveInstallPrompts(cfg, toPrompt, out, readLine); err != nil {
			return err
		}
	}
	for _, name := range toCreate {
		if err := writeEmbeddedTemplate(name); err != nil {
			return err
		}
	}
	for _, name := range toUpgrade {
		if err := writeEmbeddedTemplate(name); err != nil {
			return err
		}
		delete(cfg.PromptTemplates, name)
		if cfg.PromptTemplateModTime != nil {
			delete(cfg.PromptTemplateModTime, name)
		}
	}
	return config.Save(cfg)
}

func resolveInstallPrompts(cfg *config.Root, names []string, out io.Writer, readLine func(string) (string, error)) error {
	var tampered []string
	var keepCustom []string
	for _, name := range names {
		content, err := ReadTemplateFile(name)
		if err != nil {
			return err
		}
		diskSHA := SHA256Hex(content)
		if exp, ok := expectedSHA(name, cfg.PromptTemplates); ok && diskSHA == exp {
			keepCustom = append(keepCustom, name)
			continue
		}
		tampered = append(tampered, name)
	}
	if len(keepCustom) == 0 && len(tampered) == 0 {
		return nil
	}
	if len(tampered) > 0 {
		return resolveTemplatePrompts(cfg, tampered, out, readLine)
	}
	return nil
}

func planTemplateInstall(saved map[string]string) (toCreate, toUpgrade, toPrompt []string, err error) {
	names := TemplateNames()
	sort.Strings(names)
	for _, name := range names {
		emb, ok := EmbeddedTemplate(name)
		if !ok {
			continue
		}
		embSHA := SHA256Hex(emb)
		content, err := ReadTemplateFile(name)
		if err != nil {
			if os.IsNotExist(err) {
				toCreate = append(toCreate, name)
				continue
			}
			return nil, nil, nil, err
		}
		diskSHA := SHA256Hex(content)
		if diskSHA == embSHA {
			continue
		}
		if _, ok := expectedSHA(name, saved); ok {
			toPrompt = append(toPrompt, name)
			continue
		}
		toUpgrade = append(toUpgrade, name)
	}
	return toCreate, toUpgrade, toPrompt, nil
}

func writeEmbeddedTemplate(name string) error {
	emb, ok := EmbeddedTemplate(name)
	if !ok {
		return os.ErrNotExist
	}
	return WriteTemplateFile(name, emb)
}
