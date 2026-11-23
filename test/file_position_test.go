package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestProjectFilePosition(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	base := server.URL + "/__solomon/projects"
	createProject := func() string {
		t.Helper()
		var result struct{ Project struct{ ID string } }
		decodeServerTestJSON(t, postJSONForServerTest(t, base, map[string]string{"path": t.TempDir()}), &result)
		if result.Project.ID == "" {
			t.Fatal("missing project ID")
		}
		return base + "/" + result.Project.ID + "/file-position"
	}
	first, second := createProject(), createProject()
	source := "?path=notes%2Fdraft.md&view=source"
	preview := "?path=notes%2Fdraft.md&view=preview"

	put := func(route, body string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPut, route, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("file position save returned %s", response.Status)
		}
	}
	read := func(route string) *database.FilePosition {
		t.Helper()
		response, err := http.Get(route)
		if err != nil {
			t.Fatal(err)
		}
		var position *database.FilePosition
		decodeServerTestJSON(t, response, &position)
		return position
	}

	put(first+source, `{"top":280.5,"left":15,"anchor":42,"head":52,"scrollAnchor":19,"scrollOffset":-3}`)
	put(first+preview, `{"top":100,"left":20}`)
	got := read(first + source)
	if got == nil || got.Top != 280.5 || got.Left != 15 || got.Anchor != 42 || got.Head != 52 ||
		got.ScrollAnchor == nil || *got.ScrollAnchor != 19 || got.ScrollOffset == nil || *got.ScrollOffset != -3 {
		t.Fatalf("source position = %+v", got)
	}
	if got := read(first + preview); got == nil || got.Top != 100 || got.Left != 20 {
		t.Fatalf("preview position = %+v", got)
	}
	if got := read(second + source); got != nil {
		t.Fatalf("second project inherited position: %+v", got)
	}
	if got := read(first + "?path=other.txt&view=source"); got != nil {
		t.Fatalf("second file inherited position: %+v", got)
	}
}
