package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	EnvOpenAIBaseURL = "OPENAI_BASE_URL"
	EnvOpenAIAPIKey  = "OPENAI_API_KEY"
	EnvModelID       = "MODEL_ID"
	ciProviderName   = "ci-env"
)

type ExecResolveOpts struct {
	EnvFile string
}

func tomlExecReady(cfg *Root) (*Provider, bool) {
	if cfg == nil || strings.TrimSpace(cfg.Current.Model) == "" {
		return nil, false
	}
	p, err := ResolveProvider(cfg)
	if err != nil {
		return nil, false
	}
	if strings.TrimSpace(p.BaseURL) == "" || !ProviderCredentialsReady(p) {
		return nil, false
	}
	return p, true
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if cur := os.Getenv(key); cur == "" {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}

func credsFromEnv() (baseURL, apiKey, model string, ok bool) {
	baseURL = strings.TrimSpace(os.Getenv(EnvOpenAIBaseURL))
	apiKey = strings.TrimSpace(os.Getenv(EnvOpenAIAPIKey))
	model = strings.TrimSpace(os.Getenv(EnvModelID))
	ok = baseURL != "" && apiKey != "" && model != ""
	return
}

func syntheticExecConfig(baseURL, apiKey, model string) (*Root, *Provider, error) {
	norm, err := NormalizeAPIBase(baseURL)
	if err != nil {
		return nil, nil, err
	}
	p := Provider{Name: ciProviderName, BaseURL: norm, APIKey: apiKey}
	cfg := &Root{
		Current:                   Current{Provider: ciProviderName, Model: model},
		SubagentTimeoutMinutes:    DefaultSubagentTimeoutMinutes,
		CompactionThresholdTokens: DefaultCompactionThresholdTokens,
		ResponseLanguage:          DefaultResponseLanguage,
	}
	setProviderOnRoot(cfg, ciProviderName, p)
	return cfg, ProviderByName(cfg, ciProviderName), nil
}

func ResolveExecConfig(existing *Root, opts ExecResolveOpts) (*Root, *Provider, error) {
	if existing != nil {
		if p, ok := tomlExecReady(existing); ok {
			return existing, p, nil
		}
	}
	if opts.EnvFile != "" {
		if err := loadEnvFile(opts.EnvFile); err != nil {
			return nil, nil, fmt.Errorf("env-file: %w", err)
		}
	}
	baseURL, apiKey, model, ok := credsFromEnv()
	if !ok {
		return nil, nil, errors.New("missing OPENAI_BASE_URL, OPENAI_API_KEY, and MODEL_ID (config.toml incomplete and env/env-file not set)")
	}
	return syntheticExecConfig(baseURL, apiKey, model)
}
