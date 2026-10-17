package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

type article struct {
	Source, Output, Title string
	Body                  template.HTML
}
type item struct {
	Title, URL string
	Current    bool
}
type page struct {
	Title, Canonical, Source, Home, Index string
	CSS                                   template.CSS
	Body                                  template.HTML
	Navigation                            []item
}

func outputPath(name string) string {
	if path.Base(name) == "README.md" {
		return path.Join(path.Dir(name), "index.html")
	}
	if strings.EqualFold(path.Ext(name), ".md") {
		return strings.TrimSuffix(name, path.Ext(name)) + ".html"
	}
	return name
}

func relative(from, to string) string {
	result, _ := filepath.Rel(filepath.FromSlash(path.Dir(from)), filepath.FromSlash(to))
	return filepath.ToSlash(result)
}

func rewrite(raw, source string, files map[string]string, repository, revision string) string {
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Path == "" {
		return raw
	}
	target := path.Clean(path.Join(path.Dir(source), u.Path))
	if strings.HasPrefix(u.Path, "/") {
		target = path.Clean(strings.TrimPrefix(u.Path, "/"))
	}
	if target == ".." || strings.HasPrefix(target, "../") || strings.HasPrefix(u.Path, "/") {
		repoPath := path.Clean(path.Join("docs", target))
		if strings.HasPrefix(u.Path, "/") {
			repoPath = target
		}
		if repoPath == ".." || strings.HasPrefix(repoPath, "../") {
			return raw
		}
		u.Path = "/" + repository + "/blob/" + revision + "/" + repoPath
		u.Scheme, u.Host, u.RawPath = "https", "github.com", ""
		return u.String()
	}
	mapped, ok := files[target]
	if !ok {
		mapped, ok = files[path.Join(target, "README.md")]
	}
	if !ok {
		return raw
	}
	u.Path, u.RawPath = relative(outputPath(source), mapped), ""
	return u.String()
}

func headingID(label string, used map[string]bool) string {
	var result strings.Builder
	for _, r := range strings.ToLower(label) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), r == '-', r == '_':
			result.WriteRune(r)
		case unicode.IsSpace(r):
			result.WriteByte('-')
		}
	}
	base := result.String()
	id := base
	for i := 1; used[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	used[id] = true
	return id
}

func render(source []byte, name string, files map[string]string, repository, revision string) (article, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(source))
	title := strings.TrimSuffix(path.Base(name), path.Ext(name))
	ids := map[string]bool{}
	first := true
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			v.Destination = []byte(rewrite(string(v.Destination), name, files, repository, revision))
		case *ast.Image:
			v.Destination = []byte(rewrite(string(v.Destination), name, files, repository, revision))
		case *ast.Heading:
			label := string(v.Text(source))
			if first {
				title, first = label, false
			}
			v.SetAttributeString("id", []byte(headingID(label, ids)))
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return article{}, err
	}
	var body bytes.Buffer
	if err := md.Renderer().Render(&body, source, doc); err != nil {
		return article{}, err
	}
	return article{name, outputPath(name), title, template.HTML(body.String())}, nil
}

func generate(docs, output, repository, revision, base, theme string) error {
	docs, err := filepath.Abs(docs)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	for _, pair := range [][2]string{{docs, output}, {output, docs}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return fmt.Errorf("docs and output must not overlap")
		}
	}
	files := map[string]string{}
	destinations := map[string]bool{}
	err = filepath.WalkDir(docs, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in docs: %s", p)
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(docs, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		target := outputPath(name)
		if destinations[target] {
			return fmt.Errorf("output collision: %s", target)
		}
		files[name], destinations[target] = target, true
		return nil
	})
	if err != nil {
		return err
	}
	layout, err := template.ParseFiles(filepath.Join(theme, "page.html"))
	if err != nil {
		return err
	}
	css, err := os.ReadFile(filepath.Join(theme, "style.css"))
	if err != nil {
		return err
	}
	baseCSS, err := os.ReadFile("site/styles/base.css")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	articles := []article{}
	for _, name := range names {
		if !strings.EqualFold(path.Ext(name), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(docs, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		a, err := render(data, name, files, repository, revision)
		if err != nil {
			return err
		}
		articles = append(articles, a)
	}
	if err := os.RemoveAll(output); err != nil {
		return err
	}
	for _, name := range names {
		target := filepath.Join(output, filepath.FromSlash(files[name]))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if strings.EqualFold(path.Ext(name), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(docs, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
	}
	for _, a := range articles {
		navigation := []item{}
		for _, other := range articles {
			navigation = append(navigation, item{other.Title, relative(a.Output, other.Output), a.Output == other.Output})
		}
		canonical, err := url.JoinPath(base, "wiki", a.Output)
		if err != nil {
			return err
		}
		sourceURL := (&url.URL{Scheme: "https", Host: "github.com", Path: "/" + repository + "/blob/" + revision + "/docs/" + a.Source}).String()
		data := page{a.Title, canonical, sourceURL, relative(a.Output, "../index.html"), relative(a.Output, "index.html"), template.CSS(string(baseCSS) + string(css)), a.Body, navigation}
		var result bytes.Buffer
		if err := layout.Execute(&result, data); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(output, filepath.FromSlash(a.Output)), result.Bytes(), 0644); err != nil {
			return err
		}
	}
	fmt.Printf("Built wiki: %d pages, %d assets\n", len(articles), len(files)-len(articles))
	return nil
}

func main() {
	docs := flag.String("docs", "docs", "Documentation source")
	output := flag.String("output", "", "Disposable wiki output directory")
	repository := flag.String("repository", "SAPPHIR3-ROS3/Solomon", "GitHub owner/repository")
	revision := flag.String("revision", "", "Source commit revision")
	base := flag.String("base-url", "https://sapphir3-ros3.github.io/Solomon/", "Public site URL")
	theme := flag.String("theme", "scripts/site", "Template directory")
	flag.Parse()
	if *output == "" || *revision == "" {
		fmt.Fprintln(os.Stderr, "output and revision are required")
		os.Exit(1)
	}
	if err := generate(*docs, *output, *repository, *revision, *base, *theme); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
