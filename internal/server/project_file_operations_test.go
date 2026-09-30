package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectFileOperations(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "original.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []projectFileOperation{
		{Action: "copy", Path: "original.txt", Destination: "copy.txt"},
		{Action: "rename", Path: "copy.txt", Destination: "renamed.txt"},
		{Action: "move", Path: "renamed.txt", Destination: "moved.txt"},
	} {
		if err := performProjectFileOperation(root, operation); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, "moved.txt"))
	if err != nil || string(content) != "content" {
		t.Fatalf("content: %q, %v", content, err)
	}
	if err := performProjectFileOperation(root, projectFileOperation{Action: "copy", Path: "original.txt", Destination: "moved.txt"}); err == nil {
		t.Fatal("overwrote existing file")
	}
	if err := performProjectFileOperation(root, projectFileOperation{Action: "delete", Path: "moved.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "moved.txt")); !os.IsNotExist(err) {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "original.txt")); err != nil {
		t.Fatal("copy removed original")
	}
}

func TestProjectFileOperationsRejectUnsafePaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	operations := []projectFileOperation{
		{Action: "delete", Path: "."},
		{Action: "rename", Path: "file.txt", Destination: "../escape.txt"},
		{Action: "copy", Path: "file.txt", Destination: filepath.Join(outside, "escape.txt")},
		{Action: "move", Path: "file.txt", Destination: "."},
		{Action: "unknown", Path: "file.txt"},
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err == nil {
		operations = append(operations, projectFileOperation{Action: "delete", Path: "escape/outside.txt"}, projectFileOperation{Action: "copy", Path: "file.txt", Destination: "escape/copy.txt"})
	}
	for _, operation := range operations {
		if err := performProjectFileOperation(root, operation); err == nil {
			t.Fatalf("accepted unsafe operation: %+v", operation)
		}
	}
	content, err := os.ReadFile(filepath.Join(outside, "outside.txt"))
	if err != nil || string(content) != "outside" {
		t.Fatalf("modified outside file: %q, %v", content, err)
	}
}

func TestProjectFolderCopy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "source", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "nested", "file"), []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := performProjectFileOperation(root, projectFileOperation{Action: "copy", Path: "source", Destination: "source/nested/copy"}); err == nil {
		t.Fatal("allowed copy into itself")
	}
	if err := performProjectFileOperation(root, projectFileOperation{Action: "copy", Path: "source", Destination: "copy"}); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "copy", "nested", "file")); err != nil || string(content) != "nested" {
		t.Fatalf("copy failed: %q, %v", content, err)
	}
}
