package agentruntime

import (
	"context"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

func (r *Runtime) resolveSubagentRole(cfg *NestedRunConfig) error {
	if cfg == nil {
		return nil
	}
	provider := strings.TrimSpace(cfg.RoleProvider)
	model := strings.TrimSpace(cfg.RoleModel)
	if provider == "" && model == "" {
		return nil
	}
	if provider == "" || model == "" {
		return fmt.Errorf("subagent: roleProvider and roleModel must both be set")
	}
	if _, err := roles.FindSubagent(config.RolesSubagentEntries(r.Cfg), provider, model); err != nil {
		return err
	}
	cfg.RoleProvider = provider
	cfg.RoleModel = model
	return nil
}

func (r *Runtime) nestedLLMTarget(ctx context.Context, cfg NestedRunConfig) (model string, backend llm.CompletionBackend, label string, err error) {
	model = r.Model
	backend = r.Backend
	label = r.Model + "(subagent)"
	if strings.TrimSpace(cfg.RoleModel) != "" {
		model = cfg.RoleModel
		backend, err = r.backendForProvider(ctx, cfg.RoleProvider)
		if err != nil {
			return "", nil, "", err
		}
		label = cfg.RoleModel + "(subagent:" + cfg.RoleProvider + ")"
	}
	if backend == nil {
		return "", nil, "", fmt.Errorf("LLM backend not configured")
	}
	return model, backend, label, nil
}

func subagentRoleFromSession(sess *chatstore.SubSession) (provider, model string) {
	if sess == nil {
		return "", ""
	}
	return strings.TrimSpace(sess.RoleProvider), strings.TrimSpace(sess.RoleModel)
}
