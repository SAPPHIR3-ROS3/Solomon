package parent

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRetainGlobalDoesNotWaitForActiveRun(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "worker-input")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	client := &Client{in: input, out: bufio.NewReader(strings.NewReader("{\"ok\":true}\n"))}
	client.mu.Lock() // An orchestration keeps this locked while tools run.
	globalMu.Lock()
	oldClient, oldRefs, oldActive := globalClient, globalRefs, globalRunActive
	globalClient, globalRefs, globalRunActive = client, 1, false
	globalMu.Unlock()
	defer func() {
		globalMu.Lock()
		globalClient, globalRefs, globalRunActive = oldClient, oldRefs, oldActive
		globalMu.Unlock()
	}()

	done := make(chan error, 1)
	go func() { done <- RetainGlobal(context.Background()) }()
	select {
	case err := <-done:
		client.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		// Let a broken health check exit without leaving a blocked goroutine.
		client.mu.Unlock()
		<-done
		t.Fatal("retaining a worker waited for an unrelated active orchestration")
	}
	globalMu.Lock()
	refs := globalRefs
	globalMu.Unlock()
	if refs != 2 {
		t.Fatalf("retain count = %d, want 2", refs)
	}
}
