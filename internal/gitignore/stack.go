package gitignore

import (
	"os"
	"path/filepath"
	"strings"
)

type layer struct {
	base string
	m    Matcher
}

type Stack struct {
	root   string
	layers []layer
}

func NewStack(root string) *Stack {
	s := &Stack{root: filepath.Clean(root)}
	s.PushDir(s.root)
	return s
}

func (s *Stack) PushDir(absDir string) {
	absDir = filepath.Clean(absDir)
	gi := filepath.Join(absDir, ".gitignore")
	if _, err := os.Stat(gi); err != nil {
		return
	}
	m, err := NewFromFile(gi, absDir)
	if err != nil {
		return
	}
	s.layers = append(s.layers, layer{base: absDir, m: m})
}

func (s *Stack) PopDir() {
	if len(s.layers) <= 1 {
		return
	}
	s.layers = s.layers[:len(s.layers)-1]
}

func (s *Stack) Ignored(absPath string, isDir bool) bool {
	absPath = filepath.Clean(absPath)
	for _, l := range s.layers {
		if !underBase(absPath, l.base) {
			continue
		}
		if l.m.Match(absPath, isDir) {
			return true
		}
	}
	return false
}

func underBase(path, base string) bool {
	if path == base {
		return true
	}
	prefix := base + string(os.PathSeparator)
	return strings.HasPrefix(path, prefix)
}
