package tooloutput

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput/process"

func processAlive(pid int) bool {
	return process.Alive(pid)
}
