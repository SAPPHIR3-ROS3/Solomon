package prompt

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt/shellutils"

func EffectiveShell() string {
	return shellutils.Effective()
}
