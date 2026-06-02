package agent

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
)

var ErrExitChat = slash.ErrExitChat

func SlashDispatch(d commands.Deps, line string) error {
	return slash.Dispatch(d, line)
}
