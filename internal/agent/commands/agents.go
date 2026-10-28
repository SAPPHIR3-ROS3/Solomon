package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AgentNode struct {
	ID        string      `json:"id"`
	Model     string      `json:"model"`
	Reasoning string      `json:"reasoning"`
	Fast      *bool       `json:"fast"`
	Intent    string      `json:"intent"`
	Title     string      `json:"title"`
	Children  []AgentNode `json:"children"`
}

func Agents(d Deps) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(home, "run", "server", "state.json"))
	var state struct {
		URL string `json:"url"`
	}
	if err == nil {
		err = json.Unmarshal(data, &state)
	}
	if err != nil || state.URL == "" {
		port, portErr := config.ConfiguredServerPort()
		if portErr != nil {
			return portErr
		}
		state.URL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	endpoint, parseErr := url.Parse(state.URL)
	if parseErr != nil || endpoint.Scheme != "http" || endpoint.User != nil || (endpoint.Hostname() != "127.0.0.1" && endpoint.Hostname() != "localhost" && endpoint.Hostname() != "::1") {
		return fmt.Errorf("invalid local daemon URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(state.URL, "/")+"/__solomon/active-agents", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Transport: networkhttp.NewDefaultTransport(), Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("load active agents: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("load active agents: %s", response.Status)
	}
	var result struct {
		Projects []struct {
			Name   string      `json:"name"`
			Agents []AgentNode `json:"agents"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	count := 0
	for _, project := range result.Projects {
		for _, agent := range project.Agents {
			PrintAgent(d, agent, project.Name, 0)
			count++
		}
	}
	if count == 0 {
		PrintSystem(d.Out, "No agents are active right now.")
	}
	return nil
}

func PrintAgent(d Deps, node AgentNode, folder string, depth int) {
	model, reasoning, fast := node.Model, node.Reasoning, "fast=?"
	if model == "" {
		model = "unknown"
	}
	if reasoning == "" {
		reasoning = "default"
	}
	if node.Fast != nil {
		if *node.Fast {
			fast = "fast=on"
			if agentEmojiSupported() {
				fast = "⚡"
				if termcolor.Enabled() {
					fast = "[93m⚡︎[39m"
				}
			}
		} else {
			fast = "fast=off"
		}
	}
	intent := node.Intent
	if intent == "" {
		intent = node.Title
	}
	intent = strings.Join(strings.Fields(intent), " ")
	PrintSystemf(d.Out, "%s%s %s (%s %s) in %s %s", strings.Repeat("  ", depth), node.ID, model, reasoning, fast, filepath.Base(folder), intent)
	for _, child := range node.Children {
		PrintAgent(d, child, folder, depth+1)
	}
}

func agentEmojiSupported() bool {
	if term := os.Getenv("TERM"); term == "dumb" || term == "" {
		return false
	}
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if locale := os.Getenv(key); locale != "" {
			locale = strings.ToLower(locale)
			return strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")
		}
	}
	return false
}
