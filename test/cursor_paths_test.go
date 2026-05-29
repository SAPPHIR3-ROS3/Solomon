package test

import (
	"testing"

	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
)

func TestDefaultBaseURL(t *testing.T) {
	if got := cursorint.DefaultBaseURL(8766); got != "http://127.0.0.1:8766/v1/" {
		t.Fatalf("got %q", got)
	}
}

func TestInstallDirReadyFalse(t *testing.T) {
	if cursorint.InstallDirReady("") {
		t.Fatal("expected false")
	}
}
