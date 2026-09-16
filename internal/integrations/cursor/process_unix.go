//go:build !windows

package cursor

import "os/exec"

func configureSidecarProcess(_ *exec.Cmd) {}
