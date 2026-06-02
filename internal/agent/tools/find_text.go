package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pathglob"
)

func execFindText(ctx context.Context, env *Env, root string, a *findArgs) (any, error) {
	cctx, cancel := findRunContext(ctx, a)
	defer cancel()

	pat := a.Pattern
	if a.CaseInsensitive && !strings.HasPrefix(pat, "(?i)") {
		pat = "(?i)" + pat
	}
	if a.Multiline && !strings.HasPrefix(pat, "(?s)") {
		pat = "(?s)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("find: invalid pattern: %w", err)
	}
	mode := strings.TrimSpace(a.OutputMode)
	if mode == "" {
		mode = "content"
	}
	var filterGlob string
	if strings.TrimSpace(a.PathGlob) != "" {
		filterGlob = pathglob.NormalizePattern(a.PathGlob)
	}
	head := 0
	if a.HeadLimit != nil {
		head = *a.HeadLimit
	}
	candidates, err := parallelFileWalk(cctx, fileWalkOpts{
		Root:             root,
		RespectGitignore: true,
		MaxFileBytes:     maxFindFileBytes,
		SkipBinary:       true,
	})
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	var lines []string
	fileCounts := map[string]int{}
	fileSet := map[string]struct{}{}
	lim := newHeadLimiter(head)
	ctxBefore, ctxAfter := 0, 0
	if a.Context != nil && *a.Context > 0 {
		ctxBefore, ctxAfter = *a.Context, *a.Context
	}
	if a.ContextBefore != nil {
		ctxBefore = *a.ContextBefore
	}
	if a.ContextAfter != nil {
		ctxAfter = *a.ContextAfter
	}
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
					if filterGlob != "" {
						okG, err := pathglob.Match(c.RelPath, filterGlob)
						if err != nil || !okG {
							continue
						}
					}
					if env.ActivateInstructionsFromAbsPath != nil {
						env.ActivateInstructionsFromAbsPath(c.AbsPath)
					}
					scanFile(cctx, c, re, mode, ctxBefore, ctxAfter, lim, &mu, &lines, fileCounts, fileSet)
				}
			}
		}()
	}
	wg.Wait()
	if cctx.Err() != nil && len(lines) == 0 && len(fileCounts) == 0 {
		return nil, cctx.Err()
	}
	switch mode {
	case "files_with_matches":
		matches := make([]string, 0, len(fileSet))
		for p := range fileSet {
			matches = append(matches, p)
		}
		sort.Strings(matches)
		return map[string]any{
			"files":      false,
			"outputMode": mode,
			"pattern":    a.Pattern,
			"path":       relPathOrDot(env.ProjRoot, root),
			"matches":    matches,
			"count":      len(matches),
		}, nil
	case "count":
		var b strings.Builder
		keys := make([]string, 0, len(fileCounts))
		for p := range fileCounts {
			keys = append(keys, p)
		}
		sort.Strings(keys)
		for _, p := range keys {
			fmt.Fprintf(&b, "%s:%d\n", p, fileCounts[p])
		}
		return map[string]any{
			"files":      false,
			"outputMode": mode,
			"pattern":    a.Pattern,
			"path":       relPathOrDot(env.ProjRoot, root),
			"output":     strings.TrimSuffix(b.String(), "\n"),
			"exit":       0,
		}, nil
	default:
		sort.Strings(lines)
		return map[string]any{
			"files":      false,
			"outputMode": "content",
			"pattern":    a.Pattern,
			"path":       relPathOrDot(env.ProjRoot, root),
			"output":     strings.Join(lines, "\n"),
			"exit":       0,
		}, nil
	}
}

func scanFile(ctx context.Context, c fileCandidate, re *regexp.Regexp, mode string, ctxBefore, ctxAfter int, lim *headLimiter, mu *sync.Mutex, lines *[]string, fileCounts map[string]int, fileSet map[string]struct{}) {
	f, err := os.Open(c.AbsPath)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var ring []string
	fileMatched := false
	lineNo := 0
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		lineNo++
		line := sc.Text()
		ring = append(ring, line)
		if len(ring) > ctxBefore+1 {
			ring = ring[len(ring)-ctxBefore-1:]
		}
		if !re.MatchString(line) {
			continue
		}
		if !lim.allow() {
			return
		}
		fileMatched = true
		if mode == "files_with_matches" {
			mu.Lock()
			fileSet[c.RelPath] = struct{}{}
			mu.Unlock()
			return
		}
		if mode == "count" {
			mu.Lock()
			fileCounts[c.RelPath]++
			mu.Unlock()
			continue
		}
		mu.Lock()
		*lines = append(*lines, fmt.Sprintf("%s:%d:%s", c.RelPath, lineNo, line))
		for i := 0; i < ctxAfter; i++ {
			if !sc.Scan() {
				break
			}
			lineNo++
			*lines = append(*lines, fmt.Sprintf("%s:%d:%s", c.RelPath, lineNo, sc.Text()))
		}
		ring = ring[:0]
		mu.Unlock()
	}
	if mode == "count" && fileMatched {
		_ = fileMatched
	}
}
