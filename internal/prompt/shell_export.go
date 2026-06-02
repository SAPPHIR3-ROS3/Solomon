package prompt

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt/shell"

func EffectiveShell() string {
	return shell.Effective()
}
