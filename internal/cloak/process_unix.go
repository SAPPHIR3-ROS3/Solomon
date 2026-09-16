//go:build !windows

package cloak

import "os/exec"

func configureCloakProcess(_ *exec.Cmd) {}
