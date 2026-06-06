package atmention

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/gitignore"
)

const defaultIndexTTL = 30 * time.Second

type Index struct {
	Root     string
	Entries  []Entry
	LoadedAt time.Time
}

type IndexCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	cache map[string]*Index
}

func NewIndexCache() *IndexCache {
	return &IndexCache{ttl: defaultIndexTTL, cache: make(map[string]*Index)}
}

func (c *IndexCache) Get(ctx context.Context, projRoot string) ([]Entry, error) {
	root, err := filepath.Abs(projRoot)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if idx, ok := c.cache[root]; ok && time.Since(idx.LoadedAt) < c.ttl {
		out := append([]Entry(nil), idx.Entries...)
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()
	entries, err := BuildIndex(ctx, root)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cache[root] = &Index{Root: root, Entries: entries, LoadedAt: time.Now()}
	out := append([]Entry(nil), entries...)
	c.mu.Unlock()
	return out, nil
}

func (c *IndexCache) Invalidate(projRoot string) {
	root, err := filepath.Abs(projRoot)
	if err != nil {
		return
	}
	c.mu.Lock()
	delete(c.cache, root)
	c.mu.Unlock()
}

func BuildIndex(ctx context.Context, root string) ([]Entry, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	stack := gitignore.NewStack(root)
	var dirStack []string
	var out []Entry
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if path != root {
			parent := filepath.Dir(path)
			for len(dirStack) > 0 && dirStack[len(dirStack)-1] != parent {
				stack.PopDir()
				dirStack = dirStack[:len(dirStack)-1]
			}
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if skipDirName(d.Name()) {
				return filepath.SkipDir
			}
			stack.PushDir(path)
			dirStack = append(dirStack, path)
			if stack.Ignored(path, true) {
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			out = append(out, Entry{RelPath: filepath.ToSlash(rel), IsDir: true})
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if stack.Ignored(path, false) {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		out = append(out, Entry{RelPath: rel, IsDir: false})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortEntries(out)
	return out, nil
}

func sortEntries(entries []Entry) {
	for i := range entries {
		entries[i].RelPath = normalizeRel(entries[i].RelPath)
	}
	sortSlice(entries)
}

func sortSlice(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelPath < entries[j].RelPath
	})
}

func skipDirName(name string) bool {
	switch name {
	case ".git", ".hg", ".svn":
		return true
	default:
		if strings.HasPrefix(name, ".cursor") {
			return true
		}
		return false
	}
}
