package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func TestResolveExecConfigFromEnv(t *testing.T) {
	t.Setenv(config.EnvOpenAIBaseURL, "https://api.example.com/v1")
	t.Setenv(config.EnvOpenAIAPIKey, "sk-test")
	t.Setenv(config.EnvModelID, "test-model")
	cfg, p, err := config.ResolveExecConfig(nil, config.ExecResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Current.Model != "test-model" || p.APIKey != "sk-test" {
		t.Fatalf("cfg=%+v prov=%+v", cfg.Current, p)
	}
}

func TestResolveExecConfigEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ci.env")
	if err := os.WriteFile(path, []byte("OPENAI_BASE_URL=https://api.example.com/v1\nOPENAI_API_KEY=key123\nMODEL_ID=m1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvOpenAIBaseURL, "")
	t.Setenv(config.EnvOpenAIAPIKey, "")
	t.Setenv(config.EnvModelID, "")
	_, p, err := config.ResolveExecConfig(nil, config.ExecResolveOpts{EnvFile: path})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "ci-env" || p.APIKey != "key123" {
		t.Fatalf("prov %+v", p)
	}
}

func TestResolveExecConfigMissing(t *testing.T) {
	t.Setenv(config.EnvOpenAIBaseURL, "")
	t.Setenv(config.EnvOpenAIAPIKey, "")
	t.Setenv(config.EnvModelID, "")
	_, _, err := config.ResolveExecConfig(nil, config.ExecResolveOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
}
