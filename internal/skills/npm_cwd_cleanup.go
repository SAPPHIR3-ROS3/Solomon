package skills

import (
	"os"
	"path/filepath"
	"strings"
)

type npmCwdArtifactSnap struct {
	projRoot         string
	lockPath         string
	lockExisted      bool
	agentsPath       string
	agentsDirExisted bool
	cwdAgentsSkills  string
	beforeCwdSkills  map[string]int64
}

func SnapNPMCwdArtifacts(projRoot string) (npmCwdArtifactSnap, error) {
	projRoot = strings.TrimSpace(projRoot)
	if projRoot == "" {
		return npmCwdArtifactSnap{}, nil
	}
	abs, err := filepath.Abs(projRoot)
	if err != nil {
		return npmCwdArtifactSnap{}, err
	}
	snap := npmCwdArtifactSnap{projRoot: abs}
	snap.lockPath = filepath.Join(abs, "skills-lock.json")
	if _, err := os.Stat(snap.lockPath); err == nil {
		snap.lockExisted = true
	} else if !os.IsNotExist(err) {
		return npmCwdArtifactSnap{}, err
	}
	snap.agentsPath = filepath.Join(abs, ".agents")
	if fi, err := os.Stat(snap.agentsPath); err == nil && fi.IsDir() {
		snap.agentsDirExisted = true
	} else if err != nil && !os.IsNotExist(err) {
		return npmCwdArtifactSnap{}, err
	}
	snap.cwdAgentsSkills = filepath.Join(snap.agentsPath, "skills")
	before, err := snapAgentsSkills(snap.cwdAgentsSkills)
	if err != nil {
		return npmCwdArtifactSnap{}, err
	}
	snap.beforeCwdSkills = before
	return snap, nil
}

func CleanupNPMCwdArtifacts(snap npmCwdArtifactSnap, installedSkillDir string) error {
	if snap.projRoot == "" {
		return nil
	}
	if !snap.lockExisted {
		if err := os.Remove(snap.lockPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if !snap.agentsDirExisted {
		if err := os.RemoveAll(snap.agentsPath); err != nil {
			return err
		}
		return nil
	}
	after, err := snapAgentsSkills(snap.cwdAgentsSkills)
	if err != nil {
		return err
	}
	picked, err := pickImportedSkillDir(snap.beforeCwdSkills, after, installedSkillDir)
	if err != nil {
		return nil
	}
	return os.RemoveAll(filepath.Join(snap.cwdAgentsSkills, picked))
}
