package tools

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

func chatPlansDir(projectHex string) (string, error) {
	return chatstore.PlansDir(projectHex)
}
