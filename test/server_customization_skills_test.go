package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerCustomizationAddSkill(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	source := filepath.Join(t.TempDir(), "My Skill.md")
	if err := os.WriteFile(source, []byte("---\nname: Fixture skill\ndescription: Fixture description\n---\nUse the fixture.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	response := postJSONForServerTest(t, server.URL+"/__solomon/skills", map[string]string{"source": source})
	var result struct {
		Skills []struct {
			Title string `json:"title"`
			ID    string `json:"id"`
		} `json:"skills"`
	}
	decodeServerTestJSON(t, response, &result)
	found := false
	var id string
	for _, skill := range result.Skills {
		if skill.Title == "Fixture skill" {
			found = true
			id = skill.ID
		}
	}
	if !found {
		t.Fatalf("installed skill missing: %+v", result)
	}

	endpoint := server.URL + "/__solomon/skills?id=" + id
	call := func(method string, payload any) *http.Response {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, endpoint, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			defer res.Body.Close()
			t.Fatalf("%s status = %d", method, res.StatusCode)
		}
		return res
	}
	var markdown struct {
		Content string `json:"content"`
	}
	decodeServerTestJSON(t, call("GET", nil), &markdown)
	if !strings.Contains(markdown.Content, "Use the fixture.") {
		t.Fatalf("unexpected Markdown: %s", markdown.Content)
	}
	updated := "---\nname: Updated fixture\ndescription: Updated description\n---\nUpdated instructions.\n"
	decodeServerTestJSON(t, call("PATCH", map[string]string{"content": updated}), &result)
	if len(result.Skills) != 1 || result.Skills[0].Title != "Updated fixture" {
		t.Fatalf("edit not reflected in catalog: %+v", result)
	}
	decodeServerTestJSON(t, call("GET", nil), &markdown)
	if !strings.Contains(markdown.Content, "Updated instructions.") {
		t.Fatal("Markdown not saved")
	}
	decodeServerTestJSON(t, call("DELETE", nil), &result)
	if len(result.Skills) != 0 {
		t.Fatalf("skill not removed: %+v", result)
	}
	original, err := os.ReadFile(source)
	if err != nil || !strings.Contains(string(original), "Use the fixture.") {
		t.Fatal("original source changed or removed")
	}
	response = postJSONForServerTest(t, server.URL+"/__solomon/skills", map[string]string{"source": "invalid"})
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid source status = %d", response.StatusCode)
	}
}
