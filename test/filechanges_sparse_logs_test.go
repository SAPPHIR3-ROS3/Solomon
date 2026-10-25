package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
)

func TestCompareSparseLogsAndBinaryFiles(t *testing.T) {
	for _, tc := range []struct {
		name           string
		before, after  []byte
		added, removed int
		binary         bool
	}{
		{"sparse log", []byte("old\n"), append(make([]byte, 16384), []byte("new\nextra\n")...), 2, 1, false},
		{"new sparse log", nil, append(make([]byte, 16384), []byte("hello\n")...), 1, 0, false},
		{"deleted sparse log", append(make([]byte, 16384), []byte("hello\n")...), nil, 0, 1, false},
		{"binary", []byte{0, 'a'}, []byte{0, 'b'}, 0, 0, true},
		{"binary with holes", nil, append(make([]byte, 32), []byte{1, 2, '\n'}...), 0, 0, true},
		{"utf16", nil, []byte{'a', 0, '\n', 0}, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "output.log")
			if tc.before != nil {
				if err := os.WriteFile(path, tc.before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := filechanges.CapturePaths(root, []string{path})
			if err != nil {
				t.Fatal(err)
			}
			defer before.Close()
			if tc.after == nil {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, tc.after, 0600); err != nil {
				t.Fatal(err)
			}
			after, err := filechanges.CapturePaths(root, []string{path})
			if err != nil {
				t.Fatal(err)
			}
			defer after.Close()
			changes, err := filechanges.Compare(before, after)
			if err != nil {
				t.Fatal(err)
			}
			if len(changes) != 1 {
				t.Fatalf("changes: %+v", changes)
			}
			c := changes[0]
			if c.Binary != tc.binary || c.Added != tc.added || c.Removed != tc.removed {
				t.Fatalf("change: %+v", c)
			}
			if c.BeforeExists != (tc.before != nil) || c.AfterExists != (tc.after != nil) {
				t.Fatalf("existence: %+v", c)
			}
			if tc.after != nil {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, tc.after) {
					t.Fatal("source file modified", err)
				}
			}
		})
	}
}
