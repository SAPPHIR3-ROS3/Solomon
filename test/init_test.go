package test

import (
	"os"
	"testing"

	"solomon/internal/logging"
)

func TestMain(m *testing.M) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	os.Exit(m.Run())
}
