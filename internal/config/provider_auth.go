package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/auth/openai/codex"
)

const (
	ProviderNameChatGPTSub = "ChatGPT Sub"
	ProviderNameClaudeSub  = "Claude Sub"
	OpenAIPlatformBase     = "https://api.openai.com"
	AnthropicPlatformBase  = "https://api.anthropic.com"

	AuthKindAPIKey       = "api_key"
	AuthKindOAuthChatGPT = "oauth_chatgpt"
	AuthKindOAuthClaude  = "oauth_claude"

	ClaudeSubExpectedDate = "2026-06-15"
)

var chatGPTSubModelDenylistPrefixes = []string{
	"gpt-image",
	"gpt-realtime",
	"gpt-audio",
}

type OAuthTokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AccountID    string
}

func IsOAuthAuthKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case AuthKindOAuthChatGPT, AuthKindOAuthClaude:
		return true
	default:
		return false
	}
}

func (p *Provider) EffectiveAuthKind() string {
	if p == nil {
		return AuthKindAPIKey
	}
	switch strings.TrimSpace(p.AuthKind) {
	case AuthKindOAuthChatGPT:
		return AuthKindOAuthChatGPT
	case AuthKindOAuthClaude:
		return AuthKindOAuthClaude
	default:
		return AuthKindAPIKey
	}
}

func (p *Provider) IsOAuthProvider() bool {
	return p != nil && IsOAuthAuthKind(p.EffectiveAuthKind())
}

func (p *Provider) IsChatGPTSub() bool {
	return p != nil && p.Name == ProviderNameChatGPTSub && p.EffectiveAuthKind() == AuthKindOAuthChatGPT
}

func (p *Provider) IsClaudeSub() bool {
	return p != nil && p.Name == ProviderNameClaudeSub && p.EffectiveAuthKind() == AuthKindOAuthClaude
}

func (p *Provider) UsesAnthropicOAuthBearer() bool {
	return p != nil && p.IsAnthropic() && p.EffectiveAuthKind() == AuthKindOAuthClaude
}

func oauthCredentialsReady(p *Provider) bool {
	if p == nil {
		return false
	}
	if strings.TrimSpace(p.OAuthAccessToken) != "" {
		return true
	}
	return strings.TrimSpace(p.OAuthRefreshToken) != ""
}

func ProviderCredentialsReady(p *Provider) bool {
	if p == nil || strings.TrimSpace(p.BaseURL) == "" {
		return false
	}
	if p.IsOAuthProvider() {
		return oauthCredentialsReady(p)
	}
	return strings.TrimSpace(p.APIKey) != ""
}

func AppendOrUpdateProvider(r *Root, p Provider) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return
	}
	setProviderOnRoot(r, name, p)
}

func ModelPassesChatGPTSubFilter(modelID string) bool {
	m := strings.ToLower(strings.TrimSpace(modelID))
	if !strings.HasPrefix(m, "gpt") {
		return false
	}
	for _, prefix := range chatGPTSubModelDenylistPrefixes {
		if strings.HasPrefix(m, prefix) {
			return false
		}
	}
	return true
}

func ModelPassesClaudeSubFilter(modelID string) bool {
	m := strings.ToLower(strings.TrimSpace(modelID))
	return strings.HasPrefix(m, "claude-")
}

func (p *Provider) oauthExpiresAt() (time.Time, bool) {
	if p == nil {
		return time.Time{}, false
	}
	s := strings.TrimSpace(p.OAuthExpiresAt)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (p *Provider) oauthAccessExpired(now time.Time) bool {
	exp, ok := p.oauthExpiresAt()
	if !ok {
		return strings.TrimSpace(p.OAuthAccessToken) == ""
	}
	return now.Add(3 * time.Minute).After(exp)
}

func applyOAuthTokens(p *Provider, kind string, t OAuthTokenSet) {
	if p == nil {
		return
	}
	p.AuthKind = kind
	p.APIKey = ""
	p.OAuthAccessToken = t.AccessToken
	p.OAuthRefreshToken = t.RefreshToken
	if !t.ExpiresAt.IsZero() {
		p.OAuthExpiresAt = t.ExpiresAt.UTC().Format(time.RFC3339)
	} else {
		p.OAuthExpiresAt = ""
	}
	p.OAuthAccountID = t.AccountID
}

func ApplyOAuthTokens(p *Provider, t codex.TokenSet) {
	applyOAuthTokens(p, AuthKindOAuthChatGPT, OAuthTokenSet{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    t.ExpiresAt,
		AccountID:    t.AccountID,
	})
}

func ResolveProviderBearer(ctx context.Context, r *Root, p *Provider) (string, error) {
	if p == nil {
		return "", errors.New("nil provider")
	}
	switch p.EffectiveAuthKind() {
	case AuthKindOAuthChatGPT:
		return resolveChatGPTOAuthBearer(ctx, r, p)
	case AuthKindOAuthClaude:
		return "", fmt.Errorf("Claude Sub is not available yet (expected %s)", ClaudeSubExpectedDate)
	default:
		key := strings.TrimSpace(p.APIKey)
		if key == "" {
			return "", errors.New("missing API key")
		}
		return key, nil
	}
}

func resolveChatGPTOAuthBearer(ctx context.Context, r *Root, p *Provider) (string, error) {
	now := time.Now()
	if !p.oauthAccessExpired(now) {
		tok := strings.TrimSpace(p.OAuthAccessToken)
		if tok != "" {
			return tok, nil
		}
	}
	refresh := strings.TrimSpace(p.OAuthRefreshToken)
	if refresh == "" {
		return "", errors.New("ChatGPT Sub: missing OAuth tokens; run /connect")
	}
	tokens, err := codex.Refresh(ctx, refresh)
	if err != nil {
		return "", fmt.Errorf("ChatGPT Sub token refresh: %w", err)
	}
	ApplyOAuthTokens(p, tokens)
	if r != nil {
		if err := Save(r); err != nil {
			return "", err
		}
	}
	return tokens.AccessToken, nil
}

func EnsureChatGPTSubBaseURL(p *Provider) {
	if p == nil || !p.IsChatGPTSub() {
		return
	}
	if strings.Contains(strings.ToLower(p.BaseURL), "api.openai.com") {
		if norm, err := NormalizeAPIBase(codex.ChatGPTSubAPIBase); err == nil {
			p.BaseURL = norm
		}
	}
}

func EnsureClaudeSubBaseURL(p *Provider) {
	if p == nil || !p.IsClaudeSub() {
		return
	}
	if norm, err := NormalizeAnthropicBase(AnthropicPlatformBase); err == nil {
		p.BaseURL = norm
	}
}

func NewChatGPTSubProvider(baseURL string, tokens codex.TokenSet) (Provider, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = codex.ChatGPTSubAPIBase
	}
	norm, err := NormalizeAPIBase(baseURL)
	if err != nil {
		return Provider{}, err
	}
	p := Provider{
		Name:        ProviderNameChatGPTSub,
		BaseURL:     norm,
		APIProtocol: APIProtocolOpenAI,
	}
	ApplyOAuthTokens(&p, tokens)
	return p, nil
}
