package agentruntime

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func (r *Runtime) atIncludeNotifier() *atmention.Notifier {
	if r == nil {
		return nil
	}
	r.atSkipMu.Lock()
	defer r.atSkipMu.Unlock()
	if r.atSkipNotify == nil {
		r.atSkipNotify = atmention.NewNotifier()
	}
	return r.atSkipNotify
}

func (r *Runtime) mergeAtIncludeNotifier(other *atmention.Notifier) {
	if r == nil || other == nil {
		return
	}
	r.atIncludeNotifier().Merge(other)
}

func (r *Runtime) flushAtIncludeNotices() {
	if r == nil || r.machineMode() {
		return
	}
	r.atSkipMu.Lock()
	if r.atSkipNotify == nil || r.atSkipShown {
		r.atSkipMu.Unlock()
		return
	}
	r.atSkipShown = true
	msgs := r.atSkipNotify.Messages()
	r.atSkipMu.Unlock()
	if len(msgs) == 0 {
		return
	}
	termcolor.WriteSystem(r.Out, "@ include skipped:\n"+strings.Join(msgs, "\n"))
}

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
	notify := r.atIncludeNotifier()
	sections, err := r.Instructions.BuildPromptSections(context.Background(), r.ProjRoot, r.ProjHex, r.Session.ActivatedInstructionDirs, notify)
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
