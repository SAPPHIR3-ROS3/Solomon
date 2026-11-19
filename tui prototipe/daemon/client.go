package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}
type Chat struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Title     string `json:"title"`
}
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("daemon HTTP %d: %s", e.StatusCode, e.Body) }

func (c *Client) request(ctx context.Context, method, path string, body any, accept string) (*http.Response, error) {
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, input)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		return nil, &HTTPError{resp.StatusCode, strings.TrimSpace(string(data))}
	}
	return resp, nil
}

func (c *Client) CurrentProject(ctx context.Context, path string) (Project, error) {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return Project{}, err
		}
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	resp, err := c.request(ctx, http.MethodPost, "/__solomon/projects", map[string]string{"path": path}, "")
	if err != nil {
		return Project{}, err
	}
	defer resp.Body.Close()
	var result struct {
		Project Project `json:"project"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return Project{}, err
	}
	if result.Project.ID == "" {
		return Project{}, errors.New("daemon returned empty project ID")
	}
	return result.Project, nil
}

func projectPath(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, "/\\") || id == "." || id == ".." {
		return "", errors.New("invalid project ID")
	}
	return "/__solomon/projects/" + url.PathEscape(id) + "/chats", nil
}
func chatPath(project, chat string) (string, error) {
	p, err := projectPath(project)
	if err != nil {
		return "", err
	}
	if chat == "" || strings.ContainsAny(chat, "/\\") || chat == "." || chat == ".." {
		return "", errors.New("invalid chat ID")
	}
	return p + "/" + url.PathEscape(chat), nil
}

func (c *Client) CreateChat(ctx context.Context, project, title string) (Chat, error) {
	path, err := projectPath(project)
	if err != nil {
		return Chat{}, err
	}
	resp, err := c.request(ctx, http.MethodPost, path, map[string]string{"title": title}, "")
	if err != nil {
		return Chat{}, err
	}
	defer resp.Body.Close()
	var chat Chat
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&chat); err != nil {
		return Chat{}, err
	}
	if chat.ID == "" {
		return Chat{}, errors.New("daemon returned empty chat ID")
	}
	return chat, nil
}

func (c *Client) Stop(ctx context.Context, project, chat string) error {
	path, err := chatPath(project, chat)
	if err != nil {
		return err
	}
	resp, err := c.request(ctx, http.MethodPost, path+"/stop", nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return err
}
