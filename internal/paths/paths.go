package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ImagesDirName = "images"

func SolomonHome() (string, error) {
	if p := strings.TrimSpace(os.Getenv("SOLOMON_HOME")); p != "" {
		return p, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".solomon"), nil
}

func ConfigPath() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.toml"), nil
}

func MCPConfigPath() (string, error) {
	if p := os.Getenv("SOLOMON_MCP_CONFIG"); p != "" {
		return p, nil
	}
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "mcp.json"), nil
}

func ProjectsMapPath() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "projectsId.json"), nil
}

func ProjectsDir() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "projects"), nil
}

func ProjectRoot(projectHexID string) (string, error) {
	dir, err := ProjectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, projectHexID), nil
}

func SkillsRegistryPath() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "skills.json"), nil
}

func SkillsRegistryLockPath() (string, error) {
	p, err := SkillsRegistryPath()
	if err != nil {
		return "", err
	}
	return p + ".lock", nil
}

func GlobalSkillsDir() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "skills"), nil
}

func GlobalAgentsPath() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "AGENTS.md"), nil
}

func GlobalRulesDir() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "rules"), nil
}

func ProjectRulesDir(projectHexID string) (string, error) {
	proot, err := ProjectRoot(projectHexID)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "rules"), nil
}

func ProjectSkillsDir(projectHexID string) (string, error) {
	proot, err := ProjectRoot(projectHexID)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "skills"), nil
}

func LocalSkillsDir(projRoot string) string {
	return filepath.Join(projRoot, ".solomon", "skills")
}

func LocalSkillsMirrorPath(projRoot string) string {
	return filepath.Join(LocalSkillsDir(projRoot), "skills.json")
}

func SubagentsDir() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "subagents"), nil
}

func ActiveSubagentsPath() (string, error) {
	d, err := SubagentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "activeSubagents.json"), nil
}

func ScheduledSubagentPath(idHex string) (string, error) {
	d, err := SubagentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, idHex+".json"), nil
}

func EnsureSubagentsDir() error {
	d, err := SubagentsDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(d, 0o700)
}

func PromptTemplatesDir() (string, error) {
	root, err := SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "prompts", "templates"), nil
}

func EnsurePromptTemplatesDir() error {
	d, err := PromptTemplatesDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(d, 0o700)
}

func ChatImagesDir(projectHex string) (string, error) {
	proot, err := ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "chats", ImagesDirName), nil
}

func ImagePath(projectHex, chatIDHex string, seq int, t time.Time) (string, error) {
	dir, err := ChatImagesDir(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, imageFileName(chatIDHex, seq, t)), nil
}

func imageFileName(chatID string, seq int, t time.Time) string {
	iso := t.Format("2006-01-02T15-04-05.000Z07-00")
	return fmt.Sprintf("%s.%s.png", chatID, iso)
}
