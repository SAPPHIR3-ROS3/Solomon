package test

import (
	"bytes"
	"encoding/xml"
	"golang.org/x/net/html"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func pagesRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func pagesRun(t *testing.T, root string, ok bool, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	out, err := cmd.CombinedOutput()
	if (err == nil) != ok {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
func pagesRead(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func pagesWrite(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, name)
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(data), 0644); e != nil {
		t.Fatal(e)
	}
}
func pagesContains(t *testing.T, data string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(data, want) {
			t.Errorf("missing %q", want)
		}
	}
}
func pagesMissing(t *testing.T, name string) {
	t.Helper()
	if _, e := os.Lstat(name); !os.IsNotExist(e) {
		t.Fatalf("expected %s to be absent: %v", name, e)
	}
}
func pagesFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			files = append(files, p)
		}
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	return files
}

type pagesHTML struct {
	language string
	meta     map[string]string
	ids      map[string]bool
	links    []string
}

func pagesParse(t *testing.T, data string) pagesHTML {
	t.Helper()
	doc, e := html.Parse(strings.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	info := pagesHTML{meta: map[string]string{}, ids: map[string]bool{}}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if n.Data == "html" {
				info.language = attrs["lang"]
			}
			if n.Data == "meta" {
				key := attrs["name"]
				if key == "" {
					key = attrs["property"]
				}
				info.meta[key] = attrs["content"]
			}
			if id, ok := attrs["id"]; ok {
				info.ids[id] = true
			}
			for _, key := range []string{"href", "src"} {
				if raw, ok := attrs[key]; ok {
					info.links = append(info.links, raw)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return info
}
func pagesCheckLinks(t *testing.T, document string, info pagesHTML, fixture bool) {
	t.Helper()
	for _, raw := range info.links {
		link, e := url.Parse(raw)
		if e != nil {
			t.Fatalf("%s: invalid URL %q: %v", document, raw, e)
		}
		if link.IsAbs() || link.Host != "" {
			continue
		}
		target := document
		if link.Path != "" {
			target = filepath.Join(filepath.Dir(document), filepath.FromSlash(link.Path))
		}
		if fixture && strings.HasPrefix(link.Path, "../") && filepath.Base(target) == "index.html" {
			if _, e := os.Stat(target); os.IsNotExist(e) {
				continue
			}
		}
		file, e := os.Stat(target)
		if e != nil || !file.Mode().IsRegular() {
			t.Errorf("%s: broken link %q: %v", document, raw, e)
			continue
		}
		if link.Fragment != "" && filepath.Ext(target) == ".html" && !pagesParse(t, pagesRead(t, target)).ids[link.Fragment] {
			t.Errorf("%s: missing anchor for %q", document, raw)
		}
	}
}

func TestPagesRenderer(t *testing.T) {
	root := pagesRoot(t)
	bin := filepath.Join(t.TempDir(), "renderer")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	pagesRun(t, root, true, "go", "build", "-mod=readonly", "-o", bin, "./scripts/site")
	for _, tc := range []struct {
		name  string
		check func(*testing.T, string, string, func(bool, string))
	}{
		{"GFM links assets and anchors", func(t *testing.T, docs, out string, render func(bool, string)) {
			fence := strings.Repeat(string(rune(96)), 3)
			pagesWrite(t, docs, "README.md", "# Wiki & Guide\n\n[Portal](portal/README.md)\n[Page](portal/page.md?q=one#hello-world)\n[Directory](portal/)\n[Code](../cmd/main.go#L2)\n[External](https://example.com/a.md)\n![Image](images/a%20b.svg)\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\n- [x] Done\n\n~~Removed~~\n\n"+fence+"mermaid\ngraph TD; A-->B\n[Not a link](portal/page.md)\n"+fence+"\n")
			pagesWrite(t, docs, "portal/README.md", "# Portal\n\n[Home](../README.md)\n")
			pagesWrite(t, docs, "portal/page.md", "# Page\n\n## Hello *World*\n\n## Hello World\n\n## Café _name_\n\n[Self](#hello-world)\n[Asset](../images/a%20b.svg)\n[Source](../../go.mod)\n")
			pagesWrite(t, docs, "images/a b.svg", "<svg/>")
			render(true, out)
			pagesContains(t, pagesRead(t, filepath.Join(out, "index.html")), "<table>", "<del>Removed</del>", "type=\"checkbox\"", "class=\"language-mermaid\"", "[Not a link](portal/page.md)", "href=\"portal/page.html?q=one#hello-world\"", "href=\"https://github.com/owner/project/blob/abc123/cmd/main.go#L2\"", "href=\"https://example.com/a.md\"", "<h1 id=\"wiki--guide\">Wiki &amp; Guide</h1>", "href=\"https://example.com/project/wiki/index.html\"", "aria-label=\"Wiki pages\"", "aria-current=\"page\"")
			if got := pagesRead(t, filepath.Join(out, "images/a b.svg")); got != "<svg/>" {
				t.Errorf("altered asset: %q", got)
			}
			pagesContains(t, pagesRead(t, filepath.Join(out, "portal/page.html")), "id=\"hello-world-1\"", "id=\"café-name\"", "href=\"https://github.com/owner/project/blob/abc123/go.mod\"")
			for _, p := range pagesFiles(t, out) {
				if filepath.Ext(p) == ".html" {
					pagesCheckLinks(t, p, pagesParse(t, pagesRead(t, p)), true)
				}
			}
		}},
		{"modification and deletion", func(t *testing.T, docs, out string, render func(bool, string)) {
			pagesWrite(t, docs, "README.md", "# Before\n")
			pagesWrite(t, docs, "old.md", "# Old\n")
			pagesWrite(t, docs, "asset.txt", "old")
			render(true, out)
			pagesWrite(t, docs, "README.md", "# After\n")
			for _, p := range []string{"old.md", "asset.txt"} {
				if e := os.Remove(filepath.Join(docs, p)); e != nil {
					t.Fatal(e)
				}
			}
			render(true, out)
			pagesMissing(t, filepath.Join(out, "old.html"))
			pagesMissing(t, filepath.Join(out, "asset.txt"))
			data := pagesRead(t, filepath.Join(out, "index.html"))
			pagesContains(t, data, "<h1 id=\"after\">After</h1>")
			if strings.Contains(data, "old.html") {
				t.Error("stale page in navigation")
			}
		}},
		{"metadata fallback escaping and language", func(t *testing.T, docs, out string, render func(bool, string)) {
			pagesWrite(t, docs, "README.md", "# Documentation\n\nStart with **models** & \"tools\".\n")
			pagesWrite(t, docs, "new-guide.md", "# New guide\n\nUse **models** & \"tools\" with café settings.\n")
			pagesWrite(t, docs, "user-guide/interface-and-terminal.md", "# Interfaccia e terminale\n")
			render(true, out)
			data := pagesRead(t, filepath.Join(out, "new-guide.html"))
			pagesContains(t, data, "&amp;")
			info := pagesParse(t, data)
			description := "Use models & \"tools\" with café settings."
			title := "New guide — Solomon Wiki"
			for key, want := range map[string]string{"description": description, "og:description": description, "twitter:description": description, "og:url": "https://example.com/project/wiki/new-guide.html", "og:title": title, "twitter:title": title} {
				if info.meta[key] != want {
					t.Errorf("%s = %q, want %q", key, info.meta[key], want)
				}
			}
			if info.language != "en" || pagesParse(t, pagesRead(t, filepath.Join(out, "user-guide/interface-and-terminal.html"))).language != "it" {
				t.Error("incorrect page language")
			}
		}},
		{"collision preserves output", func(t *testing.T, docs, out string, render func(bool, string)) {
			pagesWrite(t, docs, "README.md", "# Index\n")
			render(true, out)
			pagesWrite(t, docs, "index.html", "collision")
			render(false, out)
			pagesContains(t, pagesRead(t, filepath.Join(out, "index.html")), "<h1 id=\"index\">Index</h1>")
		}},
		{"overlapping source rejected", func(t *testing.T, docs, out string, render func(bool, string)) {
			pagesWrite(t, docs, "README.md", "# Safe\n")
			render(false, filepath.Join(docs, "nested"))
			pagesContains(t, pagesRead(t, filepath.Join(docs, "README.md")), "# Safe")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scratch := t.TempDir()
			docs, out := filepath.Join(scratch, "docs"), filepath.Join(scratch, "output")
			tc.check(t, docs, out, func(ok bool, out string) {
				pagesRun(t, root, ok, bin, "--docs", docs, "--output", out, "--repository", "owner/project", "--revision", "abc123", "--base-url", "https://example.com/project/")
			})
		})
	}
}

func TestPagesBuilderRejectsSymlinks(t *testing.T) {
	root := pagesRoot(t)
	outside := t.TempDir()
	for _, nested := range []bool{false, true} {
		output := filepath.Join(t.TempDir(), "site")
		link := output
		if nested {
			if e := os.Mkdir(output, 0755); e != nil {
				t.Fatal(e)
			}
			link = filepath.Join(output, "wiki")
		}
		if e := os.Symlink(outside, link); e != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlink unavailable: %v", e)
			}
			t.Fatal(e)
		}
		pagesRun(t, root, false, "go", "run", "-mod=readonly", "./scripts/build_site", "--output", output)
		entries, e := os.ReadDir(outside)
		if e != nil || len(entries) != 0 {
			t.Fatalf("symlink target was changed: %v %v", entries, e)
		}
	}
}

func TestPagesBuilderActualSite(t *testing.T) {
	root, out := pagesRoot(t), t.TempDir()
	pagesRun(t, root, true, "go", "run", "-mod=readonly", "./scripts/build_site", "--output", out)
	pagesContains(t, pagesRead(t, filepath.Join(out, "index.html")), "<style>")
	nested := pagesRead(t, filepath.Join(out, "getting-started/index.html"))
	pagesContains(t, nested, "<style>", "url(\"../")
	if strings.Contains(nested, "rel=\"stylesheet\"") {
		t.Error("nested stylesheet was not inlined")
	}
	var index struct {
		XMLName   xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		Locations []string `xml:"url>loc"`
	}
	if e := xml.Unmarshal([]byte(pagesRead(t, filepath.Join(out, "sitemap.xml"))), &index); e != nil {
		t.Fatal(e)
	}
	const base = "https://sapphir3-ros3.github.io/Solomon/"
	expected := map[string]bool{base: true, base + "getting-started/": true}
	docs, wiki := filepath.Join(root, "docs"), filepath.Join(out, "wiki")
	for _, source := range pagesFiles(t, docs) {
		rel, e := filepath.Rel(docs, source)
		if e != nil {
			t.Fatal(e)
		}
		markdown := strings.EqualFold(filepath.Ext(rel), ".md")
		if filepath.Base(rel) == "README.md" {
			rel = filepath.Join(filepath.Dir(rel), "index.html")
		} else if markdown {
			rel = strings.TrimSuffix(rel, filepath.Ext(rel)) + ".html"
		}
		target := pagesRead(t, filepath.Join(wiki, rel))
		if markdown {
			expected[base+"wiki/"+filepath.ToSlash(rel)] = true
		} else if !bytes.Equal([]byte(target), []byte(pagesRead(t, source))) {
			t.Errorf("changed asset: %s", rel)
		}
	}
	for _, location := range index.Locations {
		if !expected[location] {
			t.Errorf("unexpected or duplicate sitemap URL: %s", location)
		}
		delete(expected, location)
	}
	if len(expected) != 0 {
		t.Errorf("missing sitemap URLs: %v", expected)
	}
	descriptions := map[string]bool{}
	for _, p := range pagesFiles(t, wiki) {
		if filepath.Ext(p) != ".html" {
			continue
		}
		info := pagesParse(t, pagesRead(t, p))
		m := info.meta
		d := m["description"]
		if n := utf8.RuneCountInString(d); n < 80 || n > 180 || descriptions[d] {
			t.Errorf("%s: invalid or repeated description %q", p, d)
		}
		descriptions[d] = true
		if m["og:description"] != d || m["twitter:description"] != d || m["og:title"] != m["twitter:title"] || m["og:type"] != "article" {
			t.Errorf("%s: inconsistent metadata", p)
		}
		want := "en"
		if filepath.Base(p) == "interface-and-terminal.html" {
			want = "it"
		}
		if info.language != want {
			t.Errorf("%s: language %q, want %q", p, info.language, want)
		}
		pagesCheckLinks(t, p, info, false)
	}
}
