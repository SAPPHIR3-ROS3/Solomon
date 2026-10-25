package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
)

func TestRetainGlobalDoesNotWaitForActiveRun(t *testing.T) {
	parent.CloseGlobal()
	t.Cleanup(parent.CloseGlobal)
	ctx := context.Background()
	src := `package main

import (
	"fmt"
	"sdk"
)

func main() {
	content, err := sdk.ReadFile("x.txt", "read x")
	if err != nil {
		panic(err)
	}
	fmt.Print(content)
}
`
	wasm, err := compile.BuildWASM(compile.Options{Source: src})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	runDone := make(chan error, 1)
	go func() {
		_, runErr := parent.RunGlobal(ctx, wasm, "agent", func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
			if name != "readFile" {
				return nil, fmt.Errorf("unexpected tool %q", name)
			}
			close(started)
			select {
			case <-release:
				return json.Marshal(map[string]any{"content": "payload"})
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		runDone <- runErr
	}()
	select {
	case <-started:
	case <-time.After(90 * time.Second):
		t.Fatal("orchestration did not reach the host call")
	}
	retained := make(chan error, 1)
	go func() { retained <- parent.RetainGlobal(context.Background()) }()
	select {
	case err := <-retained:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retaining a worker waited for an unrelated active orchestration")
	}
	close(release)
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}
}
