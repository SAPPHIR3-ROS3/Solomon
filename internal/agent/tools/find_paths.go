package tools

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pathglob"
)

const maxFindPathResults = 2000

type pathHit struct {
	rel   string
	mtime int64
}

func execFindPaths(ctx context.Context, env *Env, root string, a *findArgs) (any, error) {
	cctx, cancel := findRunContext(ctx, a)
	defer cancel()

	pattern := pathglob.NormalizePattern(a.Pattern)
	head := maxFindPathResults
	if a.HeadLimit != nil && *a.HeadLimit > 0 && *a.HeadLimit < head {
		head = *a.HeadLimit
	}
	candidates, err := parallelFileWalk(cctx, fileWalkOpts{
		Root:             root,
		RespectGitignore: false,
		MaxFileBytes:     maxFindFileBytes,
		SkipBinary:       false,
	})
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	hits := make([]pathHit, 0, 64)
	lim := newHeadLimiter(head)
	workers := searchWorkerCount()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-cctx.Done():
					return
				case c, ok := <-candidates:
					if !ok {
						return
					}
					okMatch, err := pathglob.Match(c.RelPath, pattern)
					if err != nil || !okMatch {
						continue
					}
					if !lim.allow() {
						return
					}
					st, err := os.Stat(c.AbsPath)
					if err != nil {
						continue
					}
					mu.Lock()
					hits = append(hits, pathHit{rel: c.RelPath, mtime: st.ModTime().UnixNano()})
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if cctx.Err() != nil && len(hits) == 0 {
		return nil, cctx.Err()
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].mtime > hits[j].mtime })
	matches := make([]string, len(hits))
	for i, h := range hits {
		matches[i] = h.rel
	}
	return map[string]any{
		"files":   true,
		"pattern": pattern,
		"path":    relPathOrDot(env.ProjRoot, root),
		"matches": matches,
		"count":   len(matches),
	}, nil
}

func relPathOrDot(projRoot, abs string) string {
	rel, err := filepath.Rel(projRoot, abs)
	if err != nil {
		return "."
	}
	if rel == "" || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}
