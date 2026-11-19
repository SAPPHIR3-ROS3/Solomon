package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func extension(path string) string {
	name := filepath.Base(path)
	if i := strings.LastIndexByte(name, '.'); i > 0 && i < len(name)-1 && strings.Trim(name[:i], ".") != "" {
		return name[i:]
	}
	return ""
}

func markdown(path string) bool { return strings.EqualFold(extension(path), ".md") }

func page(path string) string {
	if path == "README.md" {
		return "Home"
	}
	return strings.ReplaceAll(filepath.ToSlash(strings.TrimSuffix(path, extension(path))), "/", "--")
}

func resolve(path string) (string, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = cwd + string(filepath.Separator) + path
	}
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	parts := strings.Split(filepath.ToSlash(path[len(volume):]), "/")
	links := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		candidate := filepath.Join(current, part)
		info, err := os.Lstat(candidate)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			current = candidate
			continue
		}
		links++
		if links > 255 {
			return "", fmt.Errorf("symlink loop: %s", path)
		}
		target, err := os.Readlink(candidate)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			volume := filepath.VolumeName(target)
			current = volume + string(filepath.Separator)
			target = target[len(volume):]
		}
		parts = append(strings.Split(filepath.ToSlash(target), "/"), parts...)
	}
	return current, nil
}

func within(base, target string) (string, bool) {
	relative, err := filepath.Rel(base, target)
	return relative, err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func quote(path string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~/", rune(c)) {
			out.WriteByte(c)
		} else {
			out.WriteByte('%')
			out.WriteByte(hex[c>>4])
			out.WriteByte(hex[c&15])
		}
	}
	return out.String()
}

var linkPatterns = []*regexp.Regexp{
	regexp.MustCompile("(!?\\[[^\\]\\n]*\\]\\()([^\\s)]+)(\\))"),
	regexp.MustCompile("(?m)^(\\s*\\[[^\\]\\n]+\\]:\\s*)(\\S+)(.*)$"),
	regexp.MustCompile("((?:href|src)=\")([^\"]+)(\")"),
}
var fencePattern = regexp.MustCompile("^\\s{0,3}(\x60{3,}|~{3,})")
var schemePattern = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9+.-]*:")
var escapePattern = regexp.MustCompile("%[a-fA-F0-9]{2}")
var lineEnding = regexp.MustCompile("[\n\v\f\x1c-\x1e\u0085\u2028\u2029]")
var urlControls = strings.NewReplacer("\t", "", "\r", "", "\n", "")

func destination(raw, source, docs, repository, revision string) (string, error) {
	clean := urlControls.Replace(strings.TrimLeftFunc(raw, func(r rune) bool { return r <= ' ' }))
	path, fragment, _ := strings.Cut(clean, "#")
	path, query, _ := strings.Cut(path, "?")
	if path == "" || strings.HasPrefix(path, "/") || schemePattern.MatchString(path) {
		return raw, nil
	}
	path = escapePattern.ReplaceAllStringFunc(path, func(encoded string) string {
		decoded, _ := url.PathUnescape(encoded)
		return decoded
	})
	path = strings.ToValidUTF8(path, "\uFFFD")
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		path = filepath.Dir(source) + string(filepath.Separator) + path
	}
	target, err := resolve(path)
	if err != nil {
		return "", err
	}
	suffix := ""
	if query != "" {
		suffix += "?" + query
	}
	if fragment != "" {
		suffix += "#" + fragment
	}
	if relative, ok := within(docs, target); ok {
		if markdown(relative) {
			relative = page(relative)
		}
		return quote(filepath.ToSlash(relative)) + suffix, nil
	}
	relative, ok := within(filepath.Dir(docs), target)
	if !ok {
		return "", fmt.Errorf("link target outside repository: %s", target)
	}
	kind := "blob"
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		kind = "tree"
	}
	return fmt.Sprintf("https://github.com/%s/%s/%s/%s%s", repository, kind, revision, quote(filepath.ToSlash(relative)), suffix), nil
}

func rewrite(text, source, docs, repository, revision string) (string, error) {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	var result strings.Builder
	fence := ""
	start := 0
	ends := lineEnding.FindAllStringIndex(text, -1)
	ends = append(ends, []int{len(text), len(text)})
	for _, end := range ends {
		line := text[start:end[1]]
		start = end[1]
		if marker := fencePattern.FindStringSubmatch(line); marker != nil {
			token := marker[1]
			if fence == "" {
				fence = token
			} else if token[0] == fence[0] && len(token) >= len(fence) {
				fence = ""
			}
		} else if fence == "" {
			for _, pattern := range linkPatterns {
				var rewritten strings.Builder
				offset := 0
				for _, match := range pattern.FindAllStringSubmatchIndex(line, -1) {
					dest, err := destination(line[match[4]:match[5]], source, docs, repository, revision)
					if err != nil {
						return "", err
					}
					rewritten.WriteString(line[offset:match[4]])
					rewritten.WriteString(dest)
					offset = match[5]
				}
				rewritten.WriteString(line[offset:])
				line = rewritten.String()
			}
		}
		result.WriteString(line)
	}
	return result.String(), nil
}

func foldName(name string) string {
	name = strings.NewReplacer("ß", "ss", "ẞ", "ss", "İ", "i\u0307", "ﬀ", "ff", "ﬁ", "fi", "ﬂ", "fl", "ﬃ", "ffi", "ﬄ", "ffl", "ﬅ", "st", "ﬆ", "st").Replace(name)
	return strings.Map(func(r rune) rune {
		lowest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < lowest {
				lowest = next
			}
		}
		return lowest
	}, name)
}

func mirror(docs, output, repository, revision string) (int, error) {
	docs, err := resolve(docs)
	if err != nil {
		return 0, err
	}
	output, err = resolve(output)
	if err != nil {
		return 0, err
	}
	_, outputInDocs := within(docs, output)
	_, docsInOutput := within(output, docs)
	if outputInDocs || docsInOutput {
		return 0, fmt.Errorf("source and output must not overlap")
	}
	if info, err := os.Stat(filepath.Join(docs, "README.md")); err != nil || !info.Mode().IsRegular() {
		return 0, fmt.Errorf("docs/README.md is required")
	}
	type file struct {
		name    string
		content []byte
	}
	var rendered []file
	names := make(map[string]bool)
	err = filepath.WalkDir(docs, func(source string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := os.Stat(source)
		if err != nil {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not supported: %s", source)
		}
		relative, _ := within(docs, source)
		name := relative
		if markdown(relative) {
			name = page(relative) + ".md"
		}
		key := foldName(name)
		if names[key] {
			return fmt.Errorf("wiki page collision: %s", name)
		}
		names[key] = true
		content, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if markdown(relative) {
			if !utf8.Valid(content) {
				return fmt.Errorf("invalid UTF-8: %s", source)
			}
			text, err := rewrite(string(content), source, docs, repository, revision)
			if err != nil {
				return fmt.Errorf("%s: %w", source, err)
			}
			content = []byte(text)
		}
		rendered = append(rendered, file{name, content})
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(output, 0777); err != nil {
		return 0, err
	}
	children, err := os.ReadDir(output)
	if err != nil {
		return 0, err
	}
	for _, child := range children {
		if child.Name() != ".git" {
			if err := os.RemoveAll(filepath.Join(output, child.Name())); err != nil {
				return 0, err
			}
		}
	}
	for _, file := range rendered {
		target := filepath.Join(output, file.name)
		if err := os.MkdirAll(filepath.Dir(target), 0777); err != nil {
			return 0, err
		}
		if err := os.WriteFile(target, file.content, 0666); err != nil {
			return 0, err
		}
	}
	return len(rendered), nil
}

func main() {
	docs := flag.String("docs", "docs", "source documentation directory")
	output := flag.String("output", "", "output wiki directory (required)")
	repository := flag.String("repository", "", "GitHub owner/repository (required)")
	revision := flag.String("revision", "main", "Git revision for repository links")
	flag.Parse()
	provided := make(map[string]bool)
	flag.Visit(func(arg *flag.Flag) { provided[arg.Name] = true })
	if !provided["output"] || !provided["repository"] || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	count, err := mirror(*docs, *output, *repository, *revision)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Files mirrored:", count)
}
