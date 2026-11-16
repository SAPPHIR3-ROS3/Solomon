package test

import (
	"net/http"
	"os"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerDeleteResearchRemovesOnlySelectedJobAndReport(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	response := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": t.TempDir()})
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, response, &created)
	projectID := created.Project.ID
	for _, id := range []string{"failed", "retained"} {
		job := research.JobRecord{ID: id, Slug: id, Title: id, Status: research.StatusFailed, ProjectHex: projectID}
		if err := chatstore.WriteResearchJobFile(projectID, id, job); err != nil {
			t.Fatal(err)
		}
	}
	html, err := chatstore.ResearchHTMLPath(projectID, "failed")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(html, []byte("<html>Partial report</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []int{http.StatusNoContent, http.StatusNotFound} {
		request, err := http.NewRequest(http.MethodDelete, server.URL+"/__solomon/projects/"+projectID+"/research/failed", nil)
		if err != nil {
			t.Fatal(err)
		}
		result, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		result.Body.Close()
		if result.StatusCode != expected {
			t.Fatalf("delete status = %d, want %d", result.StatusCode, expected)
		}
	}
	jsonPath, err := chatstore.ResearchJobPath(projectID, "failed")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{html, jsonPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("deleted file still exists: %s (%v)", path, err)
		}
	}
	if _, err := research.GlobalManager().Get(projectID, "retained"); err != nil {
		t.Fatalf("other job changed: %v", err)
	}
}
