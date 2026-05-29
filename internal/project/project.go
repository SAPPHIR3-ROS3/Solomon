package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type MapFile map[string]string

func CanonicalRoot(abs string) (string, error) {
	clean := filepath.Clean(abs)
	real, err := filepath.EvalSymlinks(clean)
	if err != nil {
		real = clean
	}
	p, err := filepath.Abs(real)
	if err != nil {
		return "", err
	}
	return normalizeRootPath(p), nil
}

func normalizeRootPath(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS != "windows" {
		return p
	}
	if len(p) >= 2 && p[1] == ':' {
		d := p[0]
		if d >= 'a' && d <= 'z' {
			return string(d-32) + p[1:]
		}
	}
	return p
}

func IDHexFromRoot(root string) string {
	h := sha256.Sum256([]byte(root))
	return hex.EncodeToString(h[:])
}

func LoadMap(homeProjMap string) (m MapFile, err error) {
	b, err := os.ReadFile(homeProjMap)
	if err != nil {
		if os.IsNotExist(err) {
			return MapFile{}, nil
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "project map read failed", logging.LogOptions{Params: map[string]any{"path": homeProjMap, "err": err.Error()}})
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "project map unmarshal failed", logging.LogOptions{Params: map[string]any{"path": homeProjMap, "err": err.Error()}})
		return nil, err
	}
	if runtime.GOOS == "windows" && len(m) > 0 {
		out := make(MapFile, len(m))
		for k, v := range m {
			out[normalizeRootPath(k)] = v
		}
		return out, nil
	}
	return m, nil
}

func SaveMap(homeProjMap string, m MapFile) error {
	dir := filepath.Dir(homeProjMap)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "project map mkdir failed", logging.LogOptions{Params: map[string]any{"dir": dir, "err": err.Error()}})
		return err
	}
	tmp := homeProjMap + ".tmp"
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "project map marshal failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "project map write temp failed", logging.LogOptions{Params: map[string]any{"path": tmp, "err": err.Error()}})
		return err
	}
	if err := os.Rename(tmp, homeProjMap); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "project map rename failed", logging.LogOptions{Params: map[string]any{"path": homeProjMap, "err": err.Error()}})
		return err
	}
	return nil
}

func EnsureDirs(projectHex string) error {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return err
	}
	imgsDir, err := paths.ChatImagesDir(projectHex)
	if err != nil {
		return err
	}
	dirs := []string{
		filepath.Join(proot, "chats"),
		filepath.Join(proot, "chats", "subchats"),
		filepath.Join(proot, "chats", paths.ImagesDirName),
		filepath.Join(proot, "plans"),
		filepath.Join(proot, "skills"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	_ = imgsDir
	return nil
}

func Resolve(absCwd string) (rootPath string, idHex string, err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "project resolve failed", logging.LogOptions{Params: map[string]any{"cwd": absCwd, "err": err.Error()}})
		}
	}()
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
	logging.Log(logging.INFO_LOG_LEVEL, "registered new project", logging.LogOptions{Params: map[string]any{"id": idHex, "root": root}})
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
	if s == "." || s == ".." || strings.Contains(s, "..") {
		return "", os.ErrInvalid
	}
	if len(s) == 1 && (s[0] == '/' || s[0] == '\\') {
		return "", os.ErrInvalid
	}
	s = strings.ReplaceAll(s, string(os.PathSeparator), "_")
	if !strings.HasSuffix(strings.ToLower(s), ".md") {
		s += ".md"
	}
	return s, nil
}
