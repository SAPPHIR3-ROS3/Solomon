package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func TestRemoveBrokenSessionImageFiles_regressionPanicFree(t *testing.T) {
	dir := t.TempDir()
	sizes := []int{3, 4, 5}
	files := make(map[int]string)
	for i, sz := range sizes {
		path := filepath.Join(dir, "bad"+string(rune('0'+i))+".bin")
		data := make([]byte, sz)
		for j := range data {
			data[j] = byte(j + 1)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		files[i+1] = path
	}

	s := &chatstore.Session{ImageFiles: files}
	n := chatstore.RemoveBrokenSessionImageFiles(s)
	if n != 3 {
		t.Fatalf("expected 3 removed, got %d", n)
	}
	if s.ImageFiles != nil {
		t.Fatal("expected ImageFiles to be nil")
	}
	for i := range files {
		path := files[i]
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("file %s still exists", path)
		}
	}
}

func TestRemoveBrokenSessionImageFiles_validImages(t *testing.T) {
	type testCase struct {
		name string
		data []byte
	}
	cases := []testCase{
		{"png", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}},
		{"jpeg", []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}},
		{"gif87a", []byte("GIF87a")},
		{"gif89a", []byte("GIF89a")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "img.bin")
			if err := os.WriteFile(path, tc.data, 0644); err != nil {
				t.Fatal(err)
			}

			s := &chatstore.Session{ImageFiles: map[int]string{1: path}}
			n := chatstore.RemoveBrokenSessionImageFiles(s)
			if n != 0 {
				t.Fatalf("expected 0 removed, got %d", n)
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Fatal("valid image file was removed")
			}
		})
	}
}

func TestRemoveBrokenSessionImageFiles_nearGIF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "near.gif")
	if err := os.WriteFile(path, []byte("GIF88a"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &chatstore.Session{ImageFiles: map[int]string{1: path}}
	n := chatstore.RemoveBrokenSessionImageFiles(s)
	if n != 1 {
		t.Fatalf("expected 1 removed, got %d", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("near-GIF file was not removed")
	}
}

func TestRemoveBrokenSessionImageFiles_nonExistent(t *testing.T) {
	s := &chatstore.Session{ImageFiles: map[int]string{1: "/nonexistent/path/img.png"}}
	n := chatstore.RemoveBrokenSessionImageFiles(s)
	if n != 1 {
		t.Fatalf("expected 1 removed, got %d", n)
	}
	if s.ImageFiles != nil {
		t.Fatal("expected ImageFiles to be nil")
	}
}

func TestRemoveBrokenSessionImageFiles_tooSmall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.bin")
	if err := os.WriteFile(path, []byte{0x01, 0x02}, 0644); err != nil {
		t.Fatal(err)
	}

	s := &chatstore.Session{ImageFiles: map[int]string{1: path}}
	n := chatstore.RemoveBrokenSessionImageFiles(s)
	if n != 1 {
		t.Fatalf("expected 1 removed, got %d", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("too-small file was not removed")
	}
}