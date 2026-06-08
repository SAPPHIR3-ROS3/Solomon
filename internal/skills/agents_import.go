package skills

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

// AgentsSkillsRoot is ~/.agents/skills, where the npm "skills" CLI installs. That package does not allow choosing another directory, so Solomon runs installs there and copies the resulting folder into ~/.solomon/skills (or project/local paths) for the registry.

func AgentsSkillsRoot() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".agents", "skills"), nil
}

func RequireNpm(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "npm requirement check failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
	}()
	path, err := exec.LookPath("npm")
	if err != nil || path == "" {
		return npmInstallError("npm not found", nil)
	}
	cmd := exec.CommandContext(ctx, "npm", "--version")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return npmInstallError("npm is not working", err)
	}
	return nil
}

func npmInstallError(prefix string, runErr error) error {
	step := nodeInstallCommand()
	var msg string
	if strings.HasPrefix(step, "https://") {
		msg = fmt.Sprintf("%s: install Node.js from %s (needed for the skills CLI and ~/.agents/skills)", prefix, step)
	} else {
		msg = fmt.Sprintf("%s: on this system install Node.js with:\n\n\t%s\n\n(then the skills CLI can use npm/npx under ~/.agents/skills)", prefix, step)
	}
	if runErr != nil {
		return fmt.Errorf("%s\n\ncause: %w", msg, runErr)
	}
	return fmt.Errorf("%s", msg)
}

func nodeInstallCommand() string {
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("winget"); err == nil {
			return `winget install --id OpenJS.NodeJS.LTS -e --accept-source-agreements --accept-package-agreements`
		}
		if _, err := exec.LookPath("choco"); err == nil {
			return `choco install nodejs-lts -y`
		}
		return "https://nodejs.org/en/download/"
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return "brew install node"
		}
		return "https://nodejs.org/en/download/"
	case "linux":
		return linuxNodeInstallCommand()
	default:
		return "https://nodejs.org/en/download/"
	}
}

func linuxNodeInstallCommand() string {
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		id, idLike := parseOsRelease(b)
		id = strings.ToLower(strings.TrimSpace(id))
		idLike = strings.ToLower(strings.TrimSpace(idLike))
		switch {
		case id == "alpine":
			return "apk add nodejs npm"
		case id == "arch" || id == "manjaro" || id == "endeavouros":
			return "sudo pacman -S --needed nodejs npm"
		case id == "fedora" || id == "rhel" || id == "centos" || id == "rocky" || id == "almalinux":
			return "sudo dnf install -y nodejs npm"
		case strings.Contains(idLike, "rhel") || strings.Contains(idLike, "fedora"):
			return "sudo dnf install -y nodejs npm"
		case id == "opensuse-tumbleweed" || id == "opensuse-leap" || strings.HasPrefix(id, "opensuse"):
			return "sudo zypper install -y nodejs npm"
		case id == "ubuntu" || id == "debian" || id == "linuxmint" || id == "pop" || strings.Contains(idLike, "debian") || strings.Contains(idLike, "ubuntu"):
			return "sudo apt-get update && sudo apt-get install -y nodejs npm"
		}
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "sudo apt-get update && sudo apt-get install -y nodejs npm"
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return "sudo apt update && sudo apt install -y nodejs npm"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "sudo dnf install -y nodejs npm"
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return "sudo yum install -y nodejs npm"
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return "sudo pacman -S --needed nodejs npm"
	}
	if _, err := exec.LookPath("zypper"); err == nil {
		return "sudo zypper install -y nodejs npm"
	}
	if _, err := exec.LookPath("apk"); err == nil {
		return "apk add nodejs npm"
	}
	return "https://nodejs.org/en/download/package-manager/"
}

func parseOsRelease(data []byte) (id, idLike string) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch key {
		case "ID":
			id = val
		case "ID_LIKE":
			idLike = val
		}
	}
	return id, idLike
}

func snapAgentsSkills(agentsRoot string) (map[string]int64, error) {
	out := map[string]int64{}
	fi, err := os.Stat(agentsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	if !fi.IsDir() {
		return out, nil
	}
	entries, err := os.ReadDir(agentsRoot)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		st, err := os.Stat(filepath.Join(agentsRoot, e.Name()))
		if err != nil {
			continue
		}
		out[e.Name()] = st.ModTime().UnixNano()
	}
	return out, nil
}

func pickImportedSkillDir(before, after map[string]int64, preferred string) (string, error) {
	var candidates []string
	for name, t2 := range after {
		t1, existed := before[name]
		if !existed || t2 > t1 {
			candidates = append(candidates, name)
		}
	}
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		var hits []string
		for _, c := range candidates {
			if strings.EqualFold(c, preferred) {
				hits = append(hits, c)
			}
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
		if len(hits) > 1 {
			return "", fmt.Errorf("ambiguous skill folder matching %q under ~/.agents/skills", preferred)
		}
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no new or updated skill folder under ~/.agents/skills after npm; check the install command output")
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("multiple skill folders changed (%s); re-run with a single package or use a skills.sh URL with --skill", strings.Join(candidates, ", "))
	}
}

type validatedSkillsInstall struct {
	Display string
	Argv    []string
	Target  string
}

func rejectShellMeta(cmdLine string) error {
	for _, bad := range []string{"&&", "||", "|", ";", "`", "$(", ")", ">", "<"} {
		if strings.Contains(cmdLine, bad) {
			return fmt.Errorf("install command contains unsupported shell syntax: %q", bad)
		}
	}
	return nil
}

func parseSkillsInstallCommand(cmdLine string) (*validatedSkillsInstall, error) {
	cmdLine = strings.TrimSpace(cmdLine)
	if cmdLine == "" {
		return nil, fmt.Errorf("empty install command")
	}
	if err := rejectShellMeta(cmdLine); err != nil {
		return nil, err
	}
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty install command")
	}
	var argv []string
	var i int
	switch strings.ToLower(fields[0]) {
	case "npx":
		argv = append(argv, "npx")
		i = 1
		for i < len(fields) && isYesFlag(fields[i]) {
			argv = append(argv, fields[i])
			i++
		}
	case "npm":
		argv = append(argv, "npm")
		i = 1
		if i >= len(fields) || !strings.EqualFold(fields[i], "exec") {
			return nil, fmt.Errorf("only npm exec skills add ... is allowed")
		}
		argv = append(argv, fields[i])
		i++
		for i < len(fields) && isYesFlag(fields[i]) {
			argv = append(argv, fields[i])
			i++
		}
	default:
		return nil, fmt.Errorf("install command must start with npx or npm exec")
	}
	if i >= len(fields) || !strings.EqualFold(fields[i], "skills") {
		return nil, fmt.Errorf("install command must invoke the skills package")
	}
	argv = append(argv, "skills")
	i++
	if i >= len(fields) || !strings.EqualFold(fields[i], "add") {
		return nil, fmt.Errorf("install command must use skills add")
	}
	argv = append(argv, "add")
	i++
	seenSkill := false
	seenGlobal := false
	seenYes := false
	for i < len(fields) && (isGlobalFlag(fields[i]) || isYesFlag(fields[i])) {
		i++
	}
	if i >= len(fields) {
		return nil, fmt.Errorf("install command missing repository target")
	}
	target := strings.TrimSpace(fields[i])
	if target == "" {
		return nil, fmt.Errorf("install command missing repository target")
	}
	if _, err := NormalizeRepoURL(target); err != nil {
		return nil, fmt.Errorf("invalid skills install target %q: %w", target, err)
	}
	argv = append(argv, target)
	i++
	for i < len(fields) {
		tok := fields[i]
		switch {
		case isGlobalFlag(tok):
			if seenGlobal {
				return nil, fmt.Errorf("duplicate global flag in install command")
			}
			seenGlobal = true
			argv = append(argv, tok)
			i++
		case isYesFlag(tok):
			if seenYes {
				return nil, fmt.Errorf("duplicate yes flag in install command")
			}
			seenYes = true
			argv = append(argv, tok)
			i++
		case tok == "--skill":
			if seenSkill {
				return nil, fmt.Errorf("duplicate --skill in install command")
			}
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("--skill requires a value")
			}
			name := strings.TrimSpace(fields[i+1])
			if name == "" || strings.HasPrefix(name, "-") {
				return nil, fmt.Errorf("invalid --skill value")
			}
			seenSkill = true
			argv = append(argv, "--skill", name)
			i += 2
		default:
			return nil, fmt.Errorf("unsupported install command token: %q", tok)
		}
	}
	if !seenYes {
		argv = append(argv, "-y")
	}
	return &validatedSkillsInstall{Display: strings.Join(argv, " "), Argv: argv, Target: target}, nil
}

func isYesFlag(s string) bool {
	return s == "-y" || s == "--yes"
}

func isGlobalFlag(s string) bool {
	return s == "-g" || s == "--global"
}

func runInstallShellCommand(ctx context.Context, cmd string, stdout, stderr io.Writer) (err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "skill install shell command failed", logging.LogOptions{Params: map[string]any{"command": cmd, "err": err.Error()}})
		}
	}()
	validated, err := parseSkillsInstallCommand(cmd)
	if err != nil {
		return err
	}
	c := exec.CommandContext(ctx, validated.Argv[0], validated.Argv[1:]...)
	c.Stdout = stdout
	c.Stderr = stderr
	c.Stdin = nil
	return c.Run()
}

func copySkillTree(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := copySkillFile(path, target); err != nil {
			return err
		}
		return nil
	})
}

func copySkillFile(srcFile, dstFile string) error {
	if err := os.MkdirAll(filepath.Dir(dstFile), 0o755); err != nil {
		return err
	}
	srcF, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer srcF.Close()
	st, err := srcF.Stat()
	if err != nil {
		return err
	}
	tmp := dstFile + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	dstF, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, st.Mode()&0o777)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dstF, srcF); err != nil {
		dstF.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := dstF.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dstFile)
}

var reSkillsAddSplit = regexp.MustCompile(`(?i)\bskills\s+add\s+`)
var reSkillsSuffixGlobal = regexp.MustCompile(`(^|\s)(-g|--global)(\s|$)`)
var reSkillsSuffixYes = regexp.MustCompile(`(^|\s)(-y|--yes)(\s|$)`)

func EnsureSkillsAddGlobalYes(cmdLine string) string {
	validated, err := parseSkillsInstallCommand(cmdLine)
	if err != nil {
		return strings.TrimSpace(cmdLine)
	}
	return validated.Display
}

var reHTTPSGitHubCmd = regexp.MustCompile(`https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+`)
var reSkillsAddShorthand = regexp.MustCompile(`(?i)\bskills\s+add\s+([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)\b`)

func canonicalForRegistry(cmdLine string, meta *SkillsShMeta) (string, error) {
	if meta != nil && strings.TrimSpace(meta.RepoURL) != "" {
		return NormalizeRepoURL(meta.RepoURL)
	}
	if u := reHTTPSGitHubCmd.FindString(cmdLine); u != "" {
		return NormalizeRepoURL(strings.TrimSuffix(u, ".git"))
	}
	if m := reSkillsAddShorthand.FindStringSubmatch(cmdLine); len(m) == 2 {
		return NormalizeRepoURL(m[1])
	}
	return "", fmt.Errorf("could not determine GitHub repo URL for registry (expected https://github.com/... or owner/repo in the install command)")
}
