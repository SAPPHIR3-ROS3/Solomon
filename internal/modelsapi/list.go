package modelsapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ListEntry struct {
	ID string `json:"id"`
}

type listResp struct {
	Data []ListEntry `json:"data"`
}

func List(baseURL, apiKey string) ([]string, error) {
	u := strings.TrimSuffix(baseURL, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
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
	var out []string
	for _, e := range lr.Data {
		if e.ID != "" {
			out = append(out, e.ID)
		}
	}
	return out, nil
}
