package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestViteWorkerBudgetAndArgumentForwarding(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	directory := t.TempDir()
	entry := filepath.Join(directory, "node_modules", "vite", "bin", "vite.js")
	if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
		t.Fatal(err)
	}
	fixture := `console.log(JSON.stringify({threads:process.env.RAYON_NUM_THREADS, inherited:process.env.SOLOMON_VITE_TEST_VALUE,args:process.argv.slice(2)}))`
	if err := os.WriteFile(entry, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOLOMON_VITE_TEST_VALUE", "preserved")
	for _, tc := range []struct{ setting, want string }{{"", "2"}, {"4", "4"}} {
		t.Run("threads="+tc.setting, func(t *testing.T) {
			t.Setenv("RAYON_NUM_THREADS", tc.setting)
			output, err := serverruntime.ViteCommandForTest(directory, 12345).Output()
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Threads   string   `json:"threads"`
				Inherited string   `json:"inherited"`
				Args      []string `json:"args"`
			}
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if got.Threads != tc.want || got.Inherited != "preserved" || !reflect.DeepEqual(got.Args, []string{"--host", "127.0.0.1", "--port", "12345"}) {
				t.Fatalf("unexpected Vite launch: %+v", got)
			}
		})
	}
}
