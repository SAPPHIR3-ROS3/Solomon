package cursor

import "testing"

func TestDefaultBaseURL(t *testing.T) {
	if got := DefaultBaseURL(8766); got != "http://127.0.0.1:8766/v1/" {
		t.Fatalf("got %q", got)
	}
}

func TestInstallDirReadyFalse(t *testing.T) {
	if InstallDirReady("") {
		t.Fatal("expected false")
	}
}
