package agentruntime

import (
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
)

func (r *Runtime) activateInstructionsFromAbsPath(absPath string) {
	if r == nil || r.Session == nil || r.ProjRoot == "" {
		return
	}
	newDirs := instructions.ActivateDirsFromAbsPath(r.ProjRoot, absPath)
	if len(newDirs) == 0 {
		return
	}
	r.mutateSession(func(s *chatstore.Session) {
		s.ActivatedInstructionDirs = instructions.MergeActivatedDirs(s.ActivatedInstructionDirs, newDirs)
	})
}

func (r *Runtime) activateInstructionsFromShellCommand(command string) {
	if r == nil || r.ProjRoot == "" {
		return
	}
	newDirs := instructions.PathsFromShellCommand(r.ProjRoot, command)
	if len(newDirs) == 0 {
		return
	}
	r.mutateSession(func(s *chatstore.Session) {
		s.ActivatedInstructionDirs = instructions.MergeActivatedDirs(s.ActivatedInstructionDirs, newDirs)
	})
}

func (r *Runtime) instructionBlock() (string, error) {
	if r.Instructions == nil || r.Session == nil {
		return "", nil
	}
	sections, err := r.Instructions.BuildPromptSections(r.ProjRoot, r.ProjHex, r.Session.ActivatedInstructionDirs)
	if err != nil {
		return "", err
	}
	return instructions.FormatInstructionBlock(sections), nil
}

func (r *Runtime) mergeSystemWithInstructions(customSys string) (string, error) {
	block, err := r.instructionBlock()
	if err != nil {
		return "", err
	}
	customSys = strings.TrimSpace(customSys)
	block = strings.TrimSpace(block)
	if block == "" {
		return customSys, nil
	}
	if customSys == "" {
		return block, nil
	}
	return block + "\n\n" + customSys, nil
}

func (r *Runtime) activateInstructionsFromProjectRelPath(relPath string) {
	if r.ProjRoot == "" || relPath == "" {
		return
	}
	abs := filepath.Join(r.ProjRoot, filepath.FromSlash(relPath))
	r.activateInstructionsFromAbsPath(abs)
}
