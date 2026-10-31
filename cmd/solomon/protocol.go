package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelreasoning"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/acp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

type acpState struct {
	Session    *chatstore.Session                     `json:"session"`
	Provider   string                                 `json:"provider"`
	Model      string                                 `json:"model"`
	Reasoning  string                                 `json:"reasoning"`
	ModelModes map[string]map[string]config.ModelMode `json:"modelModes,omitempty"`
}

func (r *acpRunner) StopReason() string {
	switch r.runtime.LastFinishReason {
	case llm.FinishReasonLength:
		return "max_tokens"
	case llm.FinishReasonContentFilter:
		return "refusal"
	default:
		return "end_turn"
	}
}
func (r *acpRunner) Snapshot() (json.RawMessage, error) {
	sess, err := r.runtime.SnapshotSession()
	if err != nil {
		return nil, err
	}
	return json.Marshal(acpState{Session: sess, Provider: r.runtime.Prov.Name, Model: r.runtime.Model, Reasoning: r.runtime.Cfg.ReasoningEffort, ModelModes: r.runtime.Cfg.ModelModes})
}
func (r *acpRunner) Restore(data json.RawMessage) error {
	return r.RestoreContext(context.Background(), data)
}
func (r *acpRunner) RestoreContext(ctx context.Context, data json.RawMessage) error {
	var state acpState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Session == nil {
		return fmt.Errorf("missing saved conversation")
	}
	chatstore.FinishSessionLoad(state.Session)
	r.runtime.Session = state.Session
	if state.ModelModes != nil {
		r.runtime.Cfg.ModelModes = state.ModelModes
	}
	if err := r.Configure(ctx, "model", state.Provider+"/"+state.Model); err != nil {
		return err
	}
	r.runtime.Cfg.ReasoningEffort = state.Reasoning
	config.RememberCurrentModelMode(r.runtime.Cfg)
	return nil
}
func (r *acpRunner) SetPermissionHandler(fn func(context.Context, acp.Update) error) {
	r.runtime.AuthorizeTool = func(ctx context.Context, inv tooling.Invocation) error {
		var args any
		_ = json.Unmarshal(inv.Args, &args)
		return fn(ctx, acp.Update{"toolCallId": inv.ToolCallID, "title": inv.Name, "kind": "other", "status": "pending", "rawInput": args})
	}
}
func (r *acpRunner) Configure(ctx context.Context, key, value string) error {
	switch key {
	case "mode":
		switch value {
		case "code":
			r.runtime.Mode = "agent"
		case "chat":
			r.runtime.Mode = "chat"
		default:
			return fmt.Errorf("unknown mode")
		}
		return nil
	case "reasoning":
		effort := ""
		if value != "default" {
			var err error
			effort, err = config.ParseReasoningEffortToken(value)
			if err != nil {
				return err
			}
		}
		capability := r.reasoningCapability()
		if capability.Known && capability.Resolve(effort) != effort {
			return fmt.Errorf("reasoning level %q is not supported by this model", effort)
		}
		saved, err := config.LoadOptional()
		if err != nil {
			return err
		}
		if config.ProviderByName(saved, r.runtime.Prov.Name) != nil {
			if _, err = config.UpdateModelReasoning(r.runtime.Prov.Name, r.runtime.Model, effort); err != nil {
				return err
			}
		}
		r.runtime.Cfg.ReasoningEffort = effort
		config.RememberCurrentModelMode(r.runtime.Cfg)
		return nil
	case "model":
		provider, model, ok := strings.Cut(value, "/")
		if !ok {
			provider = r.runtime.Prov.Name
			model = value
		}
		p := config.ProviderByName(r.runtime.Cfg, provider)
		if p == nil || model == "" {
			return fmt.Errorf("unknown provider or model")
		}
		cfgCopy := *r.runtime.Cfg
		cfgCopy.ModelModes = cloneACPModelModes(r.runtime.Cfg.ModelModes)
		saved, err := config.LoadOptional()
		if err != nil {
			return err
		}
		if saved != nil {
			for name, modes := range saved.ModelModes {
				if cfgCopy.ModelModes[name] == nil {
					cfgCopy.ModelModes[name] = map[string]config.ModelMode{}
				}
				for id, mode := range modes {
					cfgCopy.ModelModes[name][id] = mode
				}
			}
		}
		if err := config.AssignCurrentModel(&cfgCopy, provider, model); err != nil {
			return err
		}
		backend, err := llm.NewCompletionBackend(ctx, &cfgCopy, p)
		if err != nil {
			return err
		}
		r.runtime.Cfg = &cfgCopy
		r.runtime.Prov = p
		r.runtime.Model = model
		r.runtime.Backend = backend
		if client, ok := llm.OpenAIClientFromBackend(backend); ok {
			r.runtime.Client = client
		}
		return nil
	default:
		return fmt.Errorf("unknown config option: %s", key)
	}
}
func (r *acpRunner) Configuration() map[string]any {
	current := r.runtime.Prov.Name + "/" + r.runtime.Model
	models := []any{map[string]any{"value": current, "name": current}}
	seen := map[string]bool{current: true}
	for _, id := range r.models {
		if !seen[id] {
			models = append(models, map[string]any{"value": id, "name": id})
			seen[id] = true
		}
	}
	for provider, ids := range r.runtime.Cfg.RecentModels {
		for _, model := range ids {
			id := provider + "/" + model
			if !seen[id] {
				models = append(models, map[string]any{"value": id, "name": id})
				seen[id] = true
			}
		}
	}
	capability := r.reasoningCapability()
	efforts := []any{}
	for _, id := range capability.Options() {
		efforts = append(efforts, map[string]any{"value": id, "name": id})
	}
	effort := capability.Resolve(r.runtime.Cfg.ReasoningEffort)
	r.runtime.Cfg.ReasoningEffort = effort
	reasoningName := "Reasoning effort"
	if !capability.Known {
		reasoningName += " (reasoning levels unknown)"
	}
	legacy := make([]any, 0, len(models))
	for _, option := range models {
		m := option.(map[string]any)
		legacy = append(legacy, map[string]any{"modelId": m["value"], "name": m["name"]})
	}
	options := []any{map[string]any{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": current, "options": models}}
	if len(efforts) > 0 {
		options = append(options, map[string]any{"id": "reasoning", "name": reasoningName, "category": "thought_level", "type": "select", "currentValue": effort, "options": efforts})
	}
	return map[string]any{"models": map[string]any{"currentModelId": current, "availableModels": legacy}, "configOptions": options}
}
func (r *acpRunner) PromptContent(ctx context.Context, blocks []acp.Content, emit func(acp.Update)) error {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "resource_link":
			parts = append(parts, fmt.Sprintf("Resource %s: %s", b.Name, b.URI))
		case "resource":
			resource := b.Resource
			text := resource.Text
			if resource.Blob != "" {
				data, err := base64.StdEncoding.DecodeString(resource.Blob)
				if err != nil {
					return fmt.Errorf("invalid embedded resource: %w", err)
				}
				if strings.HasPrefix(resource.MimeType, "text/") && utf8.Valid(data) {
					text = string(data)
				} else {
					path, err := saveACPAttachment(data, ".bin")
					if err != nil {
						return err
					}
					text = "Binary attachment saved at " + path
				}
			}
			parts = append(parts, fmt.Sprintf("Resource %s:\n%s", resource.URI, text))
		case "image":
			data, err := base64.StdEncoding.DecodeString(b.Data)
			if err != nil {
				return fmt.Errorf("invalid image: %w", err)
			}

			ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif"}[b.MimeType]
			if ext == "" {
				return fmt.Errorf("unsupported image MIME type: %s", b.MimeType)
			}
			path, err := saveACPAttachment(data, ext)
			if err != nil {
				return err
			}

			sess := r.runtime.Session
			seq := sess.ImageSeq
			sess.ImageSeq++
			if sess.ImageFiles == nil {
				sess.ImageFiles = map[int]string{}
			}
			sess.ImageFiles[seq] = path
			parts = append(parts, llm.ImagePlaceholder(seq, path))
		}
	}

	if opts, ok := r.Configuration()["configOptions"]; ok {
		emit(acp.Update{"sessionUpdate": "config_option_update", "configOptions": opts})
	}
	return r.Prompt(ctx, strings.Join(parts, "\n"), emit)
}

func saveACPAttachment(data []byte, ext string) (string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "acp", "attachments")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "attachment-*"+ext)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err = f.Write(data); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	if err = f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func (r *acpRunner) Commands() []any {
	return []any{map[string]any{"name": "help", "description": "Show ACP usage"}, map[string]any{"name": "compact", "description": "Summarize the conversation context"}}
}

func cloneACPModelModes(source map[string]map[string]config.ModelMode) map[string]map[string]config.ModelMode {
	result := map[string]map[string]config.ModelMode{}
	for provider, modes := range source {
		result[provider] = map[string]config.ModelMode{}
		for model, mode := range modes {
			result[provider][model] = mode
		}
	}
	return result
}

func (r *acpRunner) reasoningCapability() modelreasoning.Capability {
	return connect.ProviderReasoning(r.runtime.Prov, []string{r.runtime.Model})[r.runtime.Model]
}
