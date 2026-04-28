package project

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"crypto/sha256"

	"solomon/internal/paths"
)

type MapFile map[string]string

func CanonicalRoot(abs string) (string, error) {
	clean := filepath.Clean(abs)
	real, err := filepath.EvalSymlinks(clean)
	if err != nil {
		real = clean
	}
	return filepath.Abs(real)
}

func IDHexFromRoot(root string) string {
	h := sha256.Sum256([]byte(root))
	return hex.EncodeToString(h[:])
}

func LoadMap(homeProjMap string) (MapFile, error) {
	b, err := os.ReadFile(homeProjMap)
	if err != nil {
		if os.IsNotExist(err) {
			return MapFile{}, nil
		}
		return nil, err
	}
	var m MapFile
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func SaveMap(homeProjMap string, m MapFile) error {
	dir := filepath.Dir(homeProjMap)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := homeProjMap + ".tmp"
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, homeProjMap)
}

func EnsureDirs(projectHex string) error {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return err
	}
	dirs := []string{
		filepath.Join(proot, "chats"),
		filepath.Join(proot, "chats", "subchats"),
		filepath.Join(proot, "plans"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func Resolve(absCwd string) (rootPath string, idHex string, err error) {
	root, err := CanonicalRoot(absCwd)
	if err != nil {
		return "", "", err
	}
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return "", "", err
	}
	m, err := LoadMap(mapPath)
	if err != nil {
		return "", "", err
	}
	idHex = m[root]
	if idHex != "" && len(idHex) == 64 {
		if err := EnsureDirs(idHex); err != nil {
			return "", "", err
		}
		return root, idHex, nil
	}
	idHex = IDHexFromRoot(root)
	m[root] = idHex
	if err := SaveMap(mapPath, m); err != nil {
		return "", "", err
	}
	if err := EnsureDirs(idHex); err != nil {
		return "", "", err
	}
	return root, idHex, nil
}

func NormalizePlanName(raw string) (string, error) {
	s := filepath.Base(strings.TrimSpace(raw))
	s = filepath.Clean(s)
	s = filepath.Base(s)
	if s == "." || s == "/" || s == ".." || strings.Contains(s, "..") {
		return "", os.ErrInvalid
	}
	s = strings.ReplaceAll(s, string(os.PathSeparator), "_")
	if !strings.HasSuffix(strings.ToLower(s), ".md") {
		s += ".md"
	}
	return s, nil
}
