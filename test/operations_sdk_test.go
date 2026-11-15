package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

type operationsCaller struct{ env *tools.Env }

func (c operationsCaller) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	result, err := tools.Exec(ctx, c.env, "agent", tooling.Invocation{Name: name, Args: args})
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func TestOperationsSDKEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte{0, 255, 1}) }))
	defer server.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source"), []byte{0, 255}, 0o600); err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`package main
import (
 "fmt"
 "sdk"
)
func main() {
 if err := sdk.CreateDir("empty/nested", "create empty directory"); err != nil { panic(err) }
 if err := sdk.CreateDir("empty/nested", "verify directory creation is idempotent"); err != nil { panic(err) }
 directory, err := sdk.Stat("empty/nested", "verify empty directory")
 if err != nil || !directory.Exists || directory.Type != "directory" { panic("directory creation failed") }
 response, err := sdk.HTTPRequest(sdk.HTTPRequestOptions{URL:%q}, "inspect raw response")
 if err != nil { panic(err) }
 if response.Status != 200 || len(response.Body) != 3 || response.Body[1] != 255 { panic("response decoding failed") }
 if err := sdk.CopyFile("source", "copy", "copy binary fixture"); err != nil { panic(err) }
 info, err := sdk.Stat("copy", "inspect copied fixture")
 if err != nil || !info.Exists || info.Size != 2 { panic("stat failed") }
 if err := sdk.MoveFile("copy", "moved", "move fixture"); err != nil { panic(err) }
 if err := sdk.RenameFile("moved", "renamed", "rename fixture"); err != nil { panic(err) }
 if err := sdk.DeleteFile("renamed", "delete fixture"); err != nil { panic(err) }
 info, err = sdk.Stat("renamed", "verify deletion")
 if err != nil || info.Exists { panic("delete failed") }
 fmt.Print("verified")
}`, server.URL)
	wasm, err := compile.BuildWASM(compile.Options{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	runner, err := run.NewRunner(ctx, operationsCaller{env: &tools.Env{ProjRoot: root, AllowDeferredTools: true}}, run.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close(ctx)
	output, err := runner.Run(ctx, wasm)
	if err != nil || strings.TrimSpace(output) != "verified" {
		t.Fatalf("SDK execution: %q %v", output, err)
	}
}
