package modelsapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type ListEntry struct {
	ID string `json:"id"`
}

type listResp struct {
	Data []ListEntry `json:"data"`
}

type ListOpts struct {
	ChatGPTAccountID string
	UserAgent        string
	AllModels        bool
}

func List(baseURL, bearer string) ([]string, error) {
	return ListWithOpts(baseURL, bearer, ListOpts{})
}

func ListForProvider(baseURL, bearer string, apiProtocol string) ([]string, error) {
	if strings.TrimSpace(apiProtocol) == "anthropic" {
		return ListAnthropic(baseURL, bearer, false)
	}
	return List(baseURL, bearer)
}

func ListWithOpts(baseURL, bearer string, opts ListOpts) (out []string, err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "models API list failed", logging.LogOptions{Params: map[string]any{"base_url": baseURL, "err": err.Error()}})
		}
	}()
	u := strings.TrimSuffix(baseURL, "/") + "/models"
	if opts.AllModels {
		u += "?all=1"
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	if s := strings.TrimSpace(opts.UserAgent); s != "" {
		req.Header.Set("User-Agent", s)
	}
	if s := strings.TrimSpace(opts.ChatGPTAccountID); s != "" {
		req.Header.Set("ChatGPT-Account-Id", s)
	}
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models API: %s: %s", resp.Status, string(b))
	}
	var lr listResp
	if err := json.Unmarshal(b, &lr); err != nil {
		return nil, err
	}
	for _, e := range lr.Data {
		if e.ID != "" {
			out = append(out, e.ID)
		}
	}
	return out, nil
}
