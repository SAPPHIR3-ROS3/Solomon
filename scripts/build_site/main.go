package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"html"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var stylesheet = regexp.MustCompile("  <link rel=\"stylesheet\" href=\"([^\"]+)\">\n")
var cssURL = regexp.MustCompile("url\\([\"']?([^)\"']+)[\"']?\\)")
var canonical = regexp.MustCompile("<link rel=\"canonical\" href=\"([^\"]+)\">")

func resolve(path string) (string, error) {
	actual, err := filepath.EvalSymlinks(path)
	if err == nil {
		return actual, nil
	}
	parent := filepath.Dir(path)
	if !os.IsNotExist(err) || parent == path {
		return "", err
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot resolve %s", path)
	}
	actual, err = resolve(parent)
	return filepath.Join(actual, filepath.Base(path)), err
}

func within(root, name string) bool {
	rel, err := filepath.Rel(root, name)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func rejectSymlink(name string) error {
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output must not be a symlink: %s", name)
	}
	return nil
}

func localPath(dir, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

func rebase(css, source, sheet, page string) (string, error) {
	var failure error
	result := cssURL.ReplaceAllStringFunc(css, func(match string) string {
		if failure != nil {
			return match
		}
		url := cssURL.FindStringSubmatch(match)[1]
		for _, prefix := range []string{"data:", "https:", "http:", "#", "/"} {
			if strings.HasPrefix(url, prefix) {
				return match
			}
		}
		target, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(sheet), url))
		info, statErr := os.Stat(target)
		if err != nil || !within(source, target) || statErr != nil || !info.Mode().IsRegular() {
			failure = fmt.Errorf("missing CSS asset: %s (%s)", url, sheet)
			return match
		}
		rel, err := filepath.Rel(filepath.Dir(page), target)
		if err != nil {
			failure = err
			return match
		}
		return "url(\"" + filepath.ToSlash(rel) + "\")"
	})
	return result, failure
}

func inlineStyles(data, source, page string) (string, error) {
	var styles []string
	var failure error
	data = strings.ReplaceAll(strings.ReplaceAll(data, "\r\n", "\n"), "\r", "\n")
	data = stylesheet.ReplaceAllStringFunc(data, func(match string) string {
		if failure != nil {
			return match
		}
		sheet, err := filepath.EvalSymlinks(localPath(filepath.Dir(page), stylesheet.FindStringSubmatch(match)[1]))
		if err != nil {
			failure = err
			return match
		}
		if !within(source, sheet) {
			failure = fmt.Errorf("stylesheet outside site: %s", sheet)
			return match
		}
		css, err := os.ReadFile(sheet)
		if err != nil {
			failure = err
			return match
		}
		style, err := rebase(strings.ReplaceAll(strings.ReplaceAll(string(css), "\r\n", "\n"), "\r", "\n"), source, sheet, page)
		if err != nil {
			failure = err
			return match
		}
		styles = append(styles, style)
		if len(styles) == 1 {
			return "__INLINE_STYLES__\n"
		}
		return ""
	})
	if len(styles) > 0 {
		data = strings.ReplaceAll(data, "__INLINE_STYLES__", "  <style>\n"+strings.Join(styles, "\n")+"  </style>")
	}
	return data, failure
}

func copySite(source, output string) error {
	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(output, rel)
		if err := rejectSymlink(target); err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := os.Stat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported site file: %s", name)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if filepath.Ext(name) == ".html" {
			rendered, err := inlineStyles(string(data), source, name)
			if err != nil {
				return err
			}
			data = []byte(rendered)
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

type sitemapEntry struct {
	Location        string `xml:"loc"`
	LastModified    string `xml:"lastmod,omitempty"`
	ChangeFrequency string `xml:"changefreq,omitempty"`
	Priority        string `xml:"priority,omitempty"`
}
type sitemap struct {
	XMLName xml.Name       `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	Entries []sitemapEntry `xml:"url"`
}

func updateSitemap(output string) error {
	name := filepath.Join(output, "sitemap.xml")
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	var index sitemap
	if err := xml.Unmarshal(data, &index); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, entry := range index.Entries {
		seen[entry.Location] = true
	}
	err = filepath.WalkDir(filepath.Join(output, "wiki"), func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(name) != ".html" {
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if match := canonical.FindSubmatch(data); match != nil {
			location := html.UnescapeString(string(match[1]))
			if !seen[location] {
				index.Entries = append(index.Entries, sitemapEntry{Location: location})
				seen[location] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	data, err = xml.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, append([]byte(xml.Header), append(data, '\n')...), 0644)
}

func build(root, output, repository, revision, baseURL string) error {
	output, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	for _, name := range []string{output, filepath.Join(output, "wiki")} {
		if err := rejectSymlink(name); err != nil {
			return err
		}
	}
	output, err = resolve(output)
	if err != nil {
		return err
	}
	source := filepath.Join(root, "site")
	docs := filepath.Join(root, "docs")
	if within(source, output) || within(output, source) || within(docs, output) || within(output, docs) {
		return fmt.Errorf("output must not overlap the source website or documentation")
	}
	if revision == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = root
		data, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("determine revision: %w", err)
		}
		revision = strings.TrimSpace(string(data))
	}
	if err := copySite(source, output); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "-mod=readonly", "./scripts/site", "--output", filepath.Join(output, "wiki"), "--repository", repository, "--revision", revision, "--base-url", baseURL)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("render wiki: %w", err)
	}
	if err := updateSitemap(output); err != nil {
		return err
	}
	fmt.Printf("Built website: %s\n", output)
	return nil
}

func main() {
	output := flag.String("output", "", "Website output directory")
	repository := flag.String("repository", "SAPPHIR3-ROS3/Solomon", "GitHub owner/repository")
	revision := flag.String("revision", "", "Source commit revision (defaults to HEAD)")
	baseURL := flag.String("base-url", "https://sapphir3-ros3.github.io/Solomon/", "Public site URL")
	flag.Parse()
	if *output == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	root, err := os.Getwd()
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err == nil {
		err = build(root, *output, *repository, *revision, *baseURL)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
