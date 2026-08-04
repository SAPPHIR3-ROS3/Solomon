package providerui

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type ConnectRequest struct {
	APIKey  string
	BaseURL string
	Kind    int
	Name    string
}

type ConnectResponse struct {
	CurrentModel    string
	CurrentProvider string
}

// Connect runs the same provider setup used by the /connect command. The
// browser UI supplies answers to the command's existing prompts; the setup,
// validation, model discovery, authentication, and config persistence remain
// in the shared Solomon provider code.
func Connect(ctx context.Context, request ConnectRequest) (ConnectResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if request.Kind < config.ProviderKindChatGPTSub || request.Kind > config.ProviderKindCursorAPI {
		return ConnectResponse{}, fmt.Errorf("unknown provider kind %d", request.Kind)
	}
	if request.Kind == config.ProviderKindOpenAICompatible || request.Kind == config.ProviderKindAnthropicCompatible {
		if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.BaseURL) == "" || strings.TrimSpace(request.APIKey) == "" {
			return ConnectResponse{}, fmt.Errorf("provider name, base URL, and API key are required")
		}
	}
	if request.Kind == config.ProviderKindCursorAPI && strings.TrimSpace(request.APIKey) == "" {
		return ConnectResponse{}, fmt.Errorf("Cursor API key is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return ConnectResponse{}, fmt.Errorf("read config.toml: %w", err)
	}
	var output bytes.Buffer
	readLine := func(prompt string) (string, error) {
		switch {
		case strings.HasPrefix(prompt, "Select [1-5]"):
			return strconv.Itoa(request.Kind), nil
		case strings.Contains(prompt, "Display name"):
			return strings.TrimSpace(request.Name), nil
		case strings.Contains(prompt, "Base URL"):
			return strings.TrimSpace(request.BaseURL), nil
		case strings.Contains(prompt, "Cursor API key"):
			return strings.TrimSpace(request.APIKey), nil
		case strings.Contains(prompt, "API key"):
			return strings.TrimSpace(request.APIKey), nil
		case strings.Contains(prompt, "Select: 0 = keep current provider/model"):
			return "0", nil
		case strings.Contains(prompt, "Select model number"):
			return "0", nil
		case strings.Contains(prompt, "Enter index"):
			return "0", nil
		default:
			return "", fmt.Errorf("unsupported provider setup prompt: %s", prompt)
		}
	}

	err = connect.Run(connect.Deps{
		Cfg:      cfg,
		Ctx:      ctx,
		Out:      &output,
		ReadLine: readLine,
		SaveCfg: func() error {
			return config.Save(cfg)
		},
		ApplyCurrentModel: func(providerName, modelID string) error {
			cfg.Current.Provider = providerName
			cfg.Current.Model = modelID
			config.NoteRecentModelUse(cfg, providerName, modelID)
			return config.Save(cfg)
		},
		Model: func() string {
			return cfg.Current.Model
		},
		Provider: func() *config.Provider {
			return config.ProviderByName(cfg, cfg.Current.Provider)
		},
	})
	if err != nil {
		return ConnectResponse{}, err
	}
	return ConnectResponse{
		CurrentModel:    strings.TrimSpace(cfg.Current.Model),
		CurrentProvider: strings.TrimSpace(cfg.Current.Provider),
	}, nil
}
