package test

import (
	"os"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
)

func TestMain(m *testing.M) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	os.Exit(m.Run())
}
