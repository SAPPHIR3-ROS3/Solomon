package updater

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SnapshotSourceTree records the source inputs before release binaries and their
// checksums are generated. A private index leaves the user's staging untouched;
// Git applies the same ignore rules and line-ending filters as a normal commit.
func SnapshotSourceTree(root string) (string, error) {
	f, err := os.CreateTemp("", "solomon-source-index-*")
	if err != nil {
		return "", err
	}
	index := f.Name()
	f.Close()
	os.Remove(index) // read-tree needs either a valid index or a nonexistent path.
	defer os.Remove(index)
	defer os.Remove(index + ".lock")
	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("source snapshot git %s: %w: %s", args[0], err, output)
		}
		return strings.TrimSpace(string(output)), nil
	}
	if _, err := run("read-tree", "HEAD"); err != nil {
		return "", err
	}
	if _, err := run("add", "-A", "--", "."); err != nil {
		return "", err
	}
	return run("write-tree")
}
