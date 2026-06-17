package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestResearchPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	hex := "93619f1ceceeb7a95e04d2d628313536bbde0774ac260359b480be61e04b58d2"
	projRoot := filepath.Join(home, "projects", hex)
	if err := os.MkdirAll(projRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := chatstore.ResearchDir(hex)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "research" {
		t.Fatalf("dir: %q", dir)
	}
	p, err := chatstore.ResearchHTMLPath(hex, "my-topic")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "my-topic.html" {
		t.Fatalf("html: %q", p)
	}
}

func TestDeleteResearchJob(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	hex := "93619f1ceceeb7a95e04d2d628313536bbde0774ac260359b480be61e04b58d2"
	projRoot := filepath.Join(home, "projects", hex)
	if err := os.MkdirAll(projRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	slug := "my-topic"
	if err := chatstore.WriteResearchJobFile(hex, slug, map[string]string{"status": "cancelled"}); err != nil {
		t.Fatal(err)
	}
	htmlPath, err := chatstore.ResearchHTMLPath(hex, slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := chatstore.DeleteResearchJob(hex, slug); err != nil {
		t.Fatal(err)
	}
	jsonPath, _ := chatstore.ResearchJobPath(hex, slug)
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Fatalf("json still exists: %v", err)
	}
	if _, err := os.Stat(htmlPath); !os.IsNotExist(err) {
		t.Fatalf("html still exists: %v", err)
	}
}
