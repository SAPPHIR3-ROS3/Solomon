package test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestProjectFolderState(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	base := server.URL + "/__solomon/projects"
	createProject := func() string {
		t.Helper()
		response := postJSONForServerTest(t, base, map[string]string{"path": t.TempDir()})
		var result struct{ Project struct{ ID string } }
		decodeServerTestJSON(t, response, &result)
		if result.Project.ID == "" {
			t.Fatal("missing project ID")
		}
		return result.Project.ID
	}
	first, second := createProject(), createProject()
	route := func(id, panel string) string { return base + "/" + id + "/folder-state?panel=" + panel }
	put := func(id, panel, body string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPut, route(id, panel), strings.NewReader(body))
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
			t.Fatalf("save returned %s", response.Status)
		}
	}
	read := func(id, panel string) *database.FolderState {
		t.Helper()
		response, err := http.Get(route(id, panel))
		if err != nil {
			t.Fatal(err)
		}
		var state *database.FolderState
		decodeServerTestJSON(t, response, &state)
		return state
	}
	put(first, "chats", `{"rootCollapsed":true,"expanded":[]}`)
	put(first, "chat-files", `{"expanded":["src","src/nested"]}`)
	put(first, "editor", `{"expanded":["docs"],"rootCollapsed":true}`)
	if got := read(first, "chat-files"); got == nil || !slices.Equal(got.Expanded, []string{"src", "src/nested"}) {
		t.Fatalf("chat files state = %+v", got)
	}
	if got := read(first, "editor"); got == nil || !got.RootCollapsed || !slices.Equal(got.Expanded, []string{"docs"}) {
		t.Fatalf("editor state = %+v", got)
	}
	if got := read(second, "chat-files"); got != nil {
		t.Fatalf("second project inherited folder state: %+v", got)
	}
	response, err := http.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	var sidebar struct {
		Projects []struct {
			ID             string
			ChatsCollapsed bool
		}
	}
	decodeServerTestJSON(t, response, &sidebar)
	if len(sidebar.Projects) != 2 {
		t.Fatalf("unexpected projects: %+v", sidebar.Projects)
	}
	for _, entry := range sidebar.Projects {
		if entry.ChatsCollapsed != (entry.ID == first) {
			t.Fatalf("unexpected chat folder state: %+v", entry)
		}
	}
	put(first, "chat-files", `{"expanded":[]}`)
	if got := read(first, "chat-files"); got == nil || got.Expanded == nil || len(got.Expanded) != 0 {
		t.Fatalf("all folders closed = %+v", got)
	}
}

func TestFolderStateMigratesExistingDatabase(t *testing.T) {
	home := t.TempDir()
	db := openTestDatabase(t, home, database.Data)
	if _, err := db.Exec(`
DROP TABLE project_folder_state;
PRAGMA user_version = 8;
INSERT INTO project_registration VALUES ('/project', 'project');
`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := database.WriteFolderState(ctx, home, "project", "editor", database.FolderState{Expanded: []string{"src"}}); err != nil {
		t.Fatal(err)
	}
	if err := database.ReleaseDataPool(home); err != nil {
		t.Fatal(err)
	}
	got, err := database.ReadFolderState(ctx, home, "project", "editor")
	if err != nil || got == nil || !slices.Equal(got.Expanded, []string{"src"}) {
		t.Fatalf("state after reopening migrated database = %+v, %v", got, err)
	}
	t.Cleanup(func() { _ = database.ReleaseDataPool(home) })
}
