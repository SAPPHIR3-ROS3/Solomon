//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultOut = "docs/loc-history.svg"

var skipDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"bundle":       true,
	"bin":          true,
	".github":      true,
	"test":         true,
	".test":        true,
}

var skipPrefixes = []string{"integrations/"}

type commitStat struct {
	totalLOC   int
	insertions int
	deletions  int
}

func main() {
	out := defaultOut
	maxCommits := 0
	repo := "."
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-out":
			i++
			if i >= len(os.Args) {
				fatal("missing value for -out")
			}
			out = os.Args[i]
		case "-max-commits":
			i++
			if i >= len(os.Args) {
				fatal("missing value for -max-commits")
			}
			n, err := strconv.Atoi(os.Args[i])
			if err != nil || n < 1 {
				fatal("invalid -max-commits")
			}
			maxCommits = n
		case "-repo":
			i++
			if i >= len(os.Args) {
				fatal("missing value for -repo")
			}
			repo = os.Args[i]
		case "-h", "--help":
			usage()
			return
		default:
			fatal("unknown argument: " + os.Args[i])
		}
	}

	root, err := gitDir(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		fatal(err)
	}
	repo = strings.TrimSpace(root)

	stats, err := collectStats(repo, maxCommits)
	if err != nil {
		fatal(err)
	}
	if len(stats) == 0 {
		fatal("no commits found")
	}

	if err := writeChart(out, stats); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d commits, last LOC %d)\n", out, len(stats), stats[len(stats)-1].totalLOC)
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: go run scripts/loc_chart.go scripts/loc_chart_render.go [options]

Counts .go and .ts lines per commit (skips node_modules,dist,bundle,bin,.github,test,.test,integrations/).

options:
  -out path          output SVG (default %s)
  -max-commits N     only the last N commits
  -repo path         git repository root (default .)
`, defaultOut)
}

func fatal(msg interface{}) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func gitDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func gitLines(dir string, args ...string) ([]string, error) {
	s, err := gitDir(dir, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func collectStats(repo string, maxCommits int) ([]commitStat, error) {
	all, err := gitLines(repo, "rev-list", "--reverse", "HEAD")
	if err != nil {
		return nil, err
	}
	hashes := all
	if maxCommits > 0 && len(hashes) > maxCommits {
		hashes = hashes[len(hashes)-maxCommits:]
	}

	inv := make(map[string]int)
	if len(hashes) > 0 && len(hashes) < len(all) {
		parent, err := gitDir(repo, "rev-parse", strings.TrimSpace(hashes[0])+"^")
		if err == nil {
			parent = strings.TrimSpace(parent)
			if parent != "" {
				if err := seedInventory(inv, repo, parent); err != nil {
					return nil, err
				}
			}
		}
	}

	stats := make([]commitStat, 0, len(hashes))
	for i, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		ns, err := diffNumstat(repo, h)
		if err != nil {
			return nil, err
		}
		if len(inv) == 0 {
			if err := seedInventory(inv, repo, h); err != nil {
				return nil, fmt.Errorf("commit %s: %w", h[:7], err)
			}
		} else if len(ns) > 0 {
			if err := applyDiff(inv, repo, h, ns); err != nil {
				return nil, fmt.Errorf("commit %s: %w", h[:7], err)
			}
		}
		if err := syncInv(inv, repo, h); err != nil {
			return nil, fmt.Errorf("commit %s: %w", h[:7], err)
		}
		add, rem := sumNumstat(ns)
		stats = append(stats, commitStat{totalLOC: sumInventory(inv), insertions: add, deletions: rem})
		fmt.Fprintf(os.Stderr, "%d/%d commits\r", i+1, len(hashes))
	}
	fmt.Fprintln(os.Stderr)
	return stats, nil
}

func syncInv(inv map[string]int, repo, commit string) error {
	files, err := gitLines(repo, "ls-tree", "-r", "--name-only", commit)
	if err != nil {
		return err
	}
	alive := make(map[string]struct{})
	var missing []string
	for _, path := range files {
		if !countPath(path) {
			continue
		}
		alive[path] = struct{}{}
		if _, ok := inv[path]; !ok {
			missing = append(missing, path)
		}
	}
	for path := range inv {
		if _, ok := alive[path]; !ok {
			delete(inv, path)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	counts, err := batchLineCounts(repo, commit, missing)
	if err != nil {
		return err
	}
	for path, n := range counts {
		if n > 0 {
			inv[path] = n
		}
	}
	return nil
}

func sumInventory(inv map[string]int) int {
	n := 0
	for _, v := range inv {
		n += v
	}
	return n
}

func seedInventory(inv map[string]int, repo, commit string) error {
	files, err := gitLines(repo, "ls-tree", "-r", "--name-only", commit)
	if err != nil {
		return err
	}
	var paths []string
	for _, path := range files {
		if countPath(path) {
			paths = append(paths, path)
		}
	}
	counts, err := batchLineCounts(repo, commit, paths)
	if err != nil {
		return err
	}
	for path, n := range counts {
		if n > 0 {
			inv[path] = n
		}
	}
	return nil
}

func applyDiff(inv map[string]int, repo, commit string, ns map[string][2]int) error {
	paths := make([]string, 0, len(ns))
	for path := range ns {
		paths = append(paths, path)
	}
	counts, err := batchLineCounts(repo, commit, paths)
	if err != nil {
		return err
	}
	for _, path := range paths {
		n, ok := counts[path]
		if !ok || n <= 0 || !countPath(path) {
			delete(inv, path)
			continue
		}
		inv[path] = n
	}
	return nil
}

func batchLineCounts(repo, commit string, paths []string) (map[string]int, error) {
	out := make(map[string]int, len(paths))
	if len(paths) == 0 {
		return out, nil
	}
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = repo
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		w := bufio.NewWriter(stdin)
		for _, p := range paths {
			fmt.Fprintf(w, "%s:%s\n", commit, p)
		}
		w.Flush()
		stdin.Close()
	}()

	br := bufio.NewReader(stdout)
	for _, path := range paths {
		hdr, err := br.ReadString('\n')
		if err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		hdr = strings.TrimSpace(hdr)
		if strings.HasSuffix(hdr, " missing") {
			continue
		}
		parts := strings.Fields(hdr)
		if len(parts) < 3 || parts[1] != "blob" {
			continue
		}
		size, err := strconv.Atoi(parts[2])
		if err != nil || size < 0 {
			_ = cmd.Wait()
			return nil, fmt.Errorf("%s: bad cat-file size %q", path, hdr)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(br, data); err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if _, err := br.ReadByte(); err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("%s: trailing newline: %w", path, err)
		}
		out[path] = lineCount(data)
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func countPath(path string) bool {
	p := filepath.ToSlash(path)
	for _, pref := range skipPrefixes {
		if strings.HasPrefix(p, pref) {
			return false
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if skipDirs[seg] {
			return false
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".go" || ext == ".ts"
}

func diffNumstat(repo, commit string) (map[string][2]int, error) {
	parents, err := gitDir(repo, "rev-list", "-n1", "--parents", commit)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(strings.TrimSpace(parents))
	var diffArgs []string
	if len(fields) <= 1 {
		diffArgs = []string{"show", "--numstat", "--format=", commit}
	} else {
		diffArgs = []string{"diff", "--numstat", fields[1] + ".." + commit}
	}
	out, err := gitDir(repo, diffArgs...)
	if err != nil {
		return nil, err
	}
	return parseNumstat(out), nil
}

func parseNumstat(out string) map[string][2]int {
	m := make(map[string][2]int)
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		add, del := 0, 0
		if parts[0] != "-" {
			add, _ = strconv.Atoi(parts[0])
		}
		if parts[1] != "-" {
			del, _ = strconv.Atoi(parts[1])
		}
		path := parts[len(parts)-1]
		if i := strings.Index(path, " => "); i >= 0 {
			old := strings.TrimSpace(path[:i])
			path = strings.TrimSpace(path[i+4:])
			if old != "" && old != path {
				m[old] = [2]int{0, 0}
			}
		}
		m[path] = [2]int{add, del}
	}
	return m
}

func sumNumstat(m map[string][2]int) (add, del int) {
	for path, v := range m {
		if !countPath(path) {
			continue
		}
		add += v[0]
		del += v[1]
	}
	return add, del
}
