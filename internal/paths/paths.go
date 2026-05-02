package paths

import (
	"os"
	"path/filepath"
)

func SolomonHome() (string, error) {
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
