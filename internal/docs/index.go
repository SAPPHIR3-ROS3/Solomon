package docs

import (
	"io/fs"
	"path"
	"strings"
	"sync"

	docscorpus "github.com/SAPPHIR3-ROS3/Solomon/v2026/docs"
	solomonembed "github.com/SAPPHIR3-ROS3/Solomon/v2026"
)

const (
	rootReadmePath    = "README.md"
	docsPortalPath    = "docs-index.md"
	sectionSplitEvery = 4
)

type chunk struct {
	path       string
	heading    string
	startLine  int
	endLine    int
	text       string
	searchText string
}

type article struct {
	path    string
	content string
	lines   int
}

type corpus struct {
	articles map[string]*article
	chunks   []chunk
}

var (
	corpusOnce sync.Once
	corpusData *corpus
	corpusErr  error
)

func loadCorpus() (*corpus, error) {
	corpusOnce.Do(func() {
		corpusData, corpusErr = buildCorpus()
	})
	return corpusData, corpusErr
}

func buildCorpus() (*corpus, error) {
	c := &corpus{articles: map[string]*article{}}
	if err := c.addArticle(rootReadmePath, solomonembed.RootReadme); err != nil {
		return nil, err
	}
	err := fs.WalkDir(docscorpus.Corpus, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == "embed_corpus.go" {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(p), ".md") {
			return nil
		}
		b, err := fs.ReadFile(docscorpus.Corpus, p)
		if err != nil {
			return err
		}
		exposed := strings.ReplaceAll(p, "\\", "/")
		if exposed == "README.md" {
			exposed = docsPortalPath
		}
		return c.addArticle(exposed, string(b))
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *corpus) addArticle(exposedPath, content string) error {
	exposedPath = normalizePath(exposedPath)
	content = strings.TrimRight(content, "\r\n")
	lines := 1
	if content != "" {
		lines = strings.Count(content, "\n") + 1
	}
	c.articles[exposedPath] = &article{path: exposedPath, content: content, lines: lines}
	for _, ch := range chunkArticle(exposedPath, content) {
		c.chunks = append(c.chunks, ch)
	}
	return nil
}

func chunkArticle(docPath, content string) []chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	var out []chunk
	var section []string
	var heading string
	sectionStart := 1
	flushSection := func(endLine int) {
		if len(section) == 0 {
			return
		}
		text := strings.Join(section, "\n")
		out = append(out, chunk{
			path:       docPath,
			heading:    heading,
			startLine:  sectionStart,
			endLine:    endLine,
			text:       text,
			searchText: docPath + "\n" + heading + "\n" + text,
		})
		section = nil
		heading = ""
	}
	for i, line := range lines {
		lineNo := i + 1
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			flushSection(lineNo - 1)
			heading = strings.TrimSpace(strings.TrimLeft(trim, "#"))
			sectionStart = lineNo
			section = []string{line}
			continue
		}
		section = append(section, line)
		if len(section) > sectionSplitEvery {
			flushSection(lineNo)
			sectionStart = lineNo + 1
		}
	}
	flushSection(len(lines))
	if len(out) == 0 {
		out = append(out, chunk{
			path:       docPath,
			heading:    "",
			startLine:  1,
			endLine:    len(lines),
			text:       content,
			searchText: docPath + "\n" + content,
		})
	}
	return out
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "docs/")
	p = strings.TrimPrefix(p, "./")
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	if p == "." {
		return ""
	}
	return p
}

func allPaths(c *corpus) []string {
	out := make([]string, 0, len(c.articles))
	for p := range c.articles {
		out = append(out, p)
	}
	return out
}
