package cloak

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

var ErrUnavailable = errors.New("CloakBrowser is unavailable")

const defaultNavigationTimeout = 30 * time.Second

//go:embed bridge.mjs
var bridgeSource []byte

type Options struct {
	InstallDir string
	CacheDir   string
	NodePath   string
	Stderr     io.Writer
}

type Browser interface {
	NewTab(context.Context, string) (string, error)
	Navigate(context.Context, string, string, string) (Navigation, error)
	Snapshot(context.Context, string, string, SnapshotOptions) (Snapshot, error)
	CloseTab(context.Context, string, string) error
	Close() error
}

type Navigation struct {
	TabID       string `json:"tabID"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
}

type SnapshotOptions struct {
	MaxCharacters int
	IncludeHTML   bool
}

type Snapshot struct {
	TabID       string `json:"tabID"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Text        string `json:"text"`
	HTML        string `json:"html"`
	Links       []Link `json:"links"`
	Truncated   bool   `json:"truncated"`
}

type Link struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type Client struct {
	mu         sync.Mutex
	installDir string
	cacheDir   string
	nodePath   string
	bridgePath string
	stderr     io.Writer

	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	nextID     uint64
	closed     bool
	processErr error
}

func NewClient(options Options) (*Client, error) {
	installDir := strings.TrimSpace(options.InstallDir)
	if installDir == "" {
		var err error
		installDir, err = paths.CloakBrowserDir()
		if err != nil {
			return nil, fmt.Errorf("%w: resolve install directory: %v", ErrUnavailable, err)
		}
	}
	packageManifest := filepath.Join(installDir, "node_modules", "cloakbrowser", "package.json")
	if _, err := os.Stat(packageManifest); err != nil {
		return nil, fmt.Errorf("%w: official npm package is not installed at %s", ErrUnavailable, installDir)
	}
	playwrightManifest := filepath.Join(installDir, "node_modules", "playwright-core", "package.json")
	if _, err := os.Stat(playwrightManifest); err != nil {
		return nil, fmt.Errorf("%w: playwright-core is not installed at %s", ErrUnavailable, installDir)
	}

	nodePath := strings.TrimSpace(options.NodePath)
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			return nil, fmt.Errorf("%w: node is not available: %v", ErrUnavailable, err)
		}
	}
	cacheDir := strings.TrimSpace(options.CacheDir)
	if cacheDir == "" {
		var err error
		cacheDir, err = paths.CloakBrowserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("%w: resolve cache directory: %v", ErrUnavailable, err)
		}
	}
	if err := os.MkdirAll(installDir, 0o700); err != nil {
		return nil, fmt.Errorf("%w: create install directory: %v", ErrUnavailable, err)
	}
	bridgePath := filepath.Join(installDir, "solomon-cloak-bridge.mjs")
	if current, err := os.ReadFile(bridgePath); err != nil || !bytes.Equal(current, bridgeSource) {
		if err := os.WriteFile(bridgePath, bridgeSource, 0o600); err != nil {
			return nil, fmt.Errorf("%w: write local bridge: %v", ErrUnavailable, err)
		}
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	return &Client{
		installDir: installDir,
		cacheDir:   cacheDir,
		nodePath:   nodePath,
		bridgePath: bridgePath,
		stderr:     options.Stderr,
	}, nil
}

func (c *Client) NewTab(ctx context.Context, intent string) (string, error) {
	var result struct {
		TabID string `json:"tabID"`
	}
	if err := c.call(ctx, intent, "newTab", nil, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.TabID) == "" {
		return "", errors.New("CloakBrowser returned an empty tab id")
	}
	return result.TabID, nil
}

func (c *Client) Navigate(ctx context.Context, tabID, rawURL, intent string) (Navigation, error) {
	var result Navigation
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := defaultNavigationTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	err := c.call(ctx, intent, "navigate", map[string]any{
		"tabID":     tabID,
		"url":       rawURL,
		"timeoutMs": maxInt64(1, timeout.Milliseconds()),
	}, &result)
	return result, err
}

func (c *Client) Snapshot(ctx context.Context, tabID, intent string, options SnapshotOptions) (Snapshot, error) {
	max := options.MaxCharacters
	if max <= 0 {
		max = 1000000
	}
	var result Snapshot
	err := c.call(ctx, intent, "snapshot", map[string]any{
		"tabID":         tabID,
		"maxCharacters": max,
		"includeHTML":   options.IncludeHTML,
	}, &result)
	if err == nil {
		parsed := ParseSnapshotHTML(result.HTML, result.URL)
		result.Title = parsed.Title
		if result.Text == "" {
			result.Text = parsed.Text
		}
		result.Links = parsed.Links
		if !options.IncludeHTML {
			result.HTML = ""
		}
	}
	return result, err
}

func (c *Client) CloseTab(ctx context.Context, tabID, intent string) error {
	return c.call(ctx, intent, "closeTab", map[string]any{"tabID": tabID}, nil)
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.stopLocked()
	return nil
}

type rpcRequest struct {
	ID        uint64 `json:"id"`
	Operation string `json:"operation"`
	Args      any    `json:"args,omitempty"`
}

type rpcResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type rpcReadResult struct {
	response rpcResponse
	err      error
}

func (c *Client) call(ctx context.Context, intent, operation string, args any, output any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrUnavailable
	}
	if err := c.startLocked(); err != nil {
		c.logCall(intent, operation, started, err)
		return err
	}
	c.nextID++
	id := c.nextID
	payload := rpcRequest{ID: id, Operation: operation, Args: args}
	if err := json.NewEncoder(c.stdin).Encode(payload); err != nil {
		err = fmt.Errorf("send Cloak operation %s: %w", operation, err)
		c.stopLocked()
		c.logCall(intent, operation, started, err)
		return err
	}
	readResult := make(chan rpcReadResult, 1)
	reader := bufio.NewReader(c.stdout)
	go func() {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			readResult <- rpcReadResult{err: err}
			return
		}
		var response rpcResponse
		err = json.Unmarshal(bytes.TrimSpace(line), &response)
		readResult <- rpcReadResult{response: response, err: err}
	}()
	select {
	case <-ctx.Done():
		c.stopLocked()
		err := ctx.Err()
		c.logCall(intent, operation, started, err)
		return err
	case read := <-readResult:
		if read.err != nil {
			err := read.err
			if c.processErr != nil {
				err = fmt.Errorf("Cloak bridge exited: %w (%v)", err, c.processErr)
			} else {
				err = fmt.Errorf("read Cloak bridge response: %w", err)
			}
			c.stopLocked()
			c.logCall(intent, operation, started, err)
			return err
		}
		if read.response.ID != id {
			err := fmt.Errorf("Cloak bridge response id mismatch: got %d, want %d", read.response.ID, id)
			c.stopLocked()
			c.logCall(intent, operation, started, err)
			return err
		}
		if message := rpcErrorMessage(read.response.Error); message != "" {
			err := fmt.Errorf("Cloak operation %s failed: %s", operation, message)
			c.logCall(intent, operation, started, err)
			return err
		}
		if output != nil && len(read.response.Result) > 0 && string(read.response.Result) != "null" {
			if err := json.Unmarshal(read.response.Result, output); err != nil {
				err = fmt.Errorf("decode Cloak operation %s response: %w", operation, err)
				c.stopLocked()
				c.logCall(intent, operation, started, err)
				return err
			}
		}
		c.logCall(intent, operation, started, nil)
		return nil
	}
}

func (c *Client) startLocked() error {
	if c.cmd != nil {
		return nil
	}
	cmd := exec.Command(c.nodePath, c.bridgePath)
	cmd.Dir = c.installDir
	cmd.Env = c.childEnvironment()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create Cloak bridge stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("create Cloak bridge stdout: %w", err)
	}
	cmd.Stderr = c.stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("start Cloak bridge: %w", err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.processErr = nil
	go c.waitProcess(cmd)
	return nil
}

func (c *Client) waitProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != cmd {
		return
	}
	c.processErr = err
	c.cmd = nil
	c.stdin = nil
	c.stdout = nil
}

func (c *Client) stopLocked() {
	cmd := c.cmd
	stdin := c.stdin
	stdout := c.stdout
	c.cmd = nil
	c.stdin = nil
	c.stdout = nil
	if stdin != nil {
		_ = stdin.Close()
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (c *Client) childEnvironment() []string {
	allowedExact := map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "LOGNAME": true,
		"TMPDIR": true, "TMP": true, "TEMP": true, "LANG": true,
		"USERPROFILE": true, "HOMEDRIVE": true, "HOMEPATH": true,
		"APPDATA": true, "LOCALAPPDATA": true, "SYSTEMROOT": true, "SystemRoot": true,
		"ComSpec": true, "PATHEXT": true, "ProgramFiles": true,
		"ProgramFiles(x86)": true,
		"DISPLAY":           true, "WAYLAND_DISPLAY": true, "HTTP_PROXY": true,
		"HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	}
	env := make([]string, 0)
	for _, item := range os.Environ() {
		name, _, ok := strings.Cut(item, "=")
		if !ok || name == "CLOAKBROWSER_LICENSE_KEY" {
			continue
		}
		if allowedExact[name] || strings.HasPrefix(name, "LC_") || strings.HasPrefix(name, "XDG_") {
			env = append(env, item)
		}
	}
	env = append(env,
		"CLOAKBROWSER_CACHE_DIR="+c.cacheDir,
		"CLOAKBROWSER_AUTO_UPDATE=false",
	)
	return env
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}

func rpcErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var message string
	if json.Unmarshal(raw, &message) == nil {
		return strings.TrimSpace(message)
	}
	var object struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &object) == nil && strings.TrimSpace(object.Message) != "" {
		return strings.TrimSpace(object.Message)
	}
	return strings.TrimSpace(string(raw))
}

func (c *Client) logCall(intent, operation string, started time.Time, err error) {
	level := logging.INFO_LOG_LEVEL
	if err != nil {
		level = logging.WARNING_LOG_LEVEL
	}
	params := map[string]any{
		"backend":    "cloak",
		"operation":  operation,
		"durationMs": time.Since(started).Milliseconds(),
	}
	if strings.TrimSpace(intent) != "" {
		params["intent"] = intent
	}
	if err != nil {
		params["err"] = err.Error()
	}
	logging.Log(level, "Cloak Browser operation", logging.LogOptions{Params: params})
}
