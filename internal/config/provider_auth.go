package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const (
	ProviderNameChatGPTSub  = "ChatGPT Sub"
	ProviderNameClaudeSub   = "Claude Sub"
	ProviderNameCursorAPI   = "Cursor API"
	ProviderNameCursorSub   = "Cursor Sub"
	CursorAPIDefaultModelID = "composer-2.5"
	OpenAIPlatformBase      = "https://api.openai.com"
	AnthropicPlatformBase   = "https://api.anthropic.com"

	AuthKindAPIKey       = "api_key"
	AuthKindOAuthChatGPT = "oauth_chatgpt"
	AuthKindOAuthClaude  = "oauth_claude"
	AuthKindOAuthCursor  = "oauth_cursor"
	AuthKindCursorAPI    = "cursor_api"

	AnthropicClaudeCodeOAuthTokenPrefix = "sk-ant-oat"

	AnthropicClaudeCodeOAuthSetupWarning = "Claude Code OAuth tokens (sk-ant-oat…) are not recommended as API keys."
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
	case AuthKindOAuthChatGPT, AuthKindOAuthClaude, AuthKindOAuthCursor:
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
	case AuthKindOAuthCursor:
		return AuthKindOAuthCursor
	case AuthKindCursorAPI:
		return AuthKindCursorAPI
	default:
		return AuthKindAPIKey
	}
}

func (p *Provider) IsCursorAPI() bool {
	return p != nil && p.Name != ProviderNameCursorSub && p.EffectiveAuthKind() == AuthKindCursorAPI
}

func (p *Provider) IsCursorSub() bool {
	return p != nil && p.Name == ProviderNameCursorSub && p.EffectiveAuthKind() == AuthKindOAuthCursor
}

func migrateMisnamedCursorSub(r *Root) {
	if r == nil || r.Providers == nil {
		return
	}
	p := r.Providers[ProviderNameCursorAPI]
	if p != nil && p.EffectiveAuthKind() == AuthKindOAuthCursor && r.Providers[ProviderNameCursorSub] == nil {
		sub := *p
		sub.Name = ProviderNameCursorSub
		sub.APIKey = ""
		r.Providers[ProviderNameCursorSub] = &sub
	}
	dropCursorAPIIfSubPresent(r)
	if sub := r.Providers[ProviderNameCursorSub]; sub != nil {
		EnsureCursorSubBaseURL(sub)
	}
}

func dropCursorAPIIfSubPresent(r *Root) {
	if r == nil || r.Providers == nil || r.Providers[ProviderNameCursorSub] == nil {
		return
	}
	if r.Providers[ProviderNameCursorAPI] == nil {
		return
	}
	remapProviderName(r, ProviderNameCursorAPI, ProviderNameCursorSub)
	delete(r.Providers, ProviderNameCursorAPI)
}

func remapProviderName(r *Root, from, to string) {
	if r == nil || from == "" || to == "" || from == to {
		return
	}
	if strings.TrimSpace(r.Current.Provider) == from {
		r.Current.Provider = to
	}
	if r.RecentModels != nil {
		if models, ok := r.RecentModels[from]; ok {
			r.RecentModels[to] = append(append([]string{}, r.RecentModels[to]...), models...)
			delete(r.RecentModels, from)
		}
	}
	if r.HiddenModels != nil {
		if models, ok := r.HiddenModels[from]; ok {
			r.HiddenModels[to] = append(append([]string{}, r.HiddenModels[to]...), models...)
			delete(r.HiddenModels, from)
		}
	}
}

func CursorAPIConfigured(r *Root) bool {
	if r == nil {
		return false
	}
	for _, p := range ProviderList(r) {
		if p.IsCursorAPI() && ProviderCredentialsReady(&p) {
			return true
		}
	}
	return false
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

func IsAnthropicClaudeCodeOAuthToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), AnthropicClaudeCodeOAuthTokenPrefix)
}

func (p *Provider) UsesAnthropicOAuthBearer() bool {
	if p == nil || !p.IsAnthropic() {
		return false
	}
	if p.EffectiveAuthKind() == AuthKindOAuthClaude {
		return true
	}
	return IsAnthropicClaudeCodeOAuthToken(p.APIKey)
}

func WriteAnthropicClaudeCodeOAuthWarning(out io.Writer) {
	if out == nil {
		return
	}
	termcolor.WriteSystem(out, AnthropicClaudeCodeOAuthSetupWarning)
}

func oauthCredentialsReady(p *Provider) bool {
	if p == nil {
		return false
	}
	if p.EffectiveAuthKind() == AuthKindOAuthCursor {
		return cursorSessionToken(p) != "" || strings.TrimSpace(p.OAuthRefreshToken) != "" || strings.TrimSpace(p.APIKey) != ""
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
	if p.IsCursorAPI() {
		return strings.TrimSpace(p.APIKey) != ""
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
	if m == "" {
		return false
	}
	for _, prefix := range chatGPTSubModelDenylistPrefixes {
		if strings.HasPrefix(m, prefix) {
			return false
		}
	}
	if strings.HasPrefix(m, "gpt") {
		return true
	}
	for _, kw := range []string{"sol", "terra", "luna", "codex"} {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

func ModelPassesChatGPTSubPickerFilter(modelID string) bool {
	if !ModelPassesChatGPTSubFilter(modelID) {
		return false
	}
	for _, seg := range strings.Split(strings.ToLower(strings.TrimSpace(modelID)), "-") {
		if seg == "pro" {
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

func ApplyClaudeOAuthTokens(p *Provider, t claude.TokenSet) {
	applyOAuthTokens(p, AuthKindOAuthClaude, OAuthTokenSet{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    t.ExpiresAt,
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
		return resolveClaudeOAuthBearer(ctx, r, p)
	case AuthKindOAuthCursor:
		return resolveCursorOAuthBearer(ctx, r, p)
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
		logging.Log(logging.ERROR_LOG_LEVEL, "ChatGPT Sub OAuth refresh token missing", logging.LogOptions{Params: map[string]any{"provider": p.Name}})
		return "", errors.New("ChatGPT Sub: missing OAuth tokens; run /connect")
	}
	tokens, err := codex.Refresh(ctx, refresh)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "ChatGPT Sub OAuth token refresh failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
		return "", fmt.Errorf("ChatGPT Sub token refresh: %w", err)
	}
	ApplyOAuthTokens(p, tokens)
	if r != nil {
		if err := Save(r); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "save config after OAuth refresh failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
			return "", err
		}
	}
	return tokens.AccessToken, nil
}

func resolveClaudeOAuthBearer(ctx context.Context, r *Root, p *Provider) (string, error) {
	now := time.Now()
	if !p.oauthAccessExpired(now) {
		tok := strings.TrimSpace(p.OAuthAccessToken)
		if tok != "" {
			return tok, nil
		}
	}
	refresh := strings.TrimSpace(p.OAuthRefreshToken)
	if refresh == "" {
		logging.Log(logging.ERROR_LOG_LEVEL, "Claude Sub OAuth refresh token missing", logging.LogOptions{Params: map[string]any{"provider": p.Name}})
		return "", errors.New("Claude Sub: missing OAuth tokens; run /connect")
	}
	tokens, err := claude.Refresh(ctx, refresh)
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "Claude Sub OAuth token refresh failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
		return "", fmt.Errorf("Claude Sub token refresh: %w", err)
	}
	ApplyClaudeOAuthTokens(p, tokens)
	if r != nil {
		if err := Save(r); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "save config after Claude OAuth refresh failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
			return "", err
		}
	}
	return tokens.AccessToken, nil
}

func ApplyCursorOAuthTokens(p *Provider, t cursorauth.TokenSet) {
	keepKey := strings.TrimSpace(p.APIKey)
	if strings.TrimSpace(t.APIKey) != "" {
		keepKey = strings.TrimSpace(t.APIKey)
	}
	applyOAuthTokens(p, AuthKindOAuthCursor, OAuthTokenSet{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    t.ExpiresAt,
	})
	p.APIKey = keepKey
}

func ResolveCursorSessionBearer(ctx context.Context, r *Root, p *Provider) (string, error) {
	return resolveCursorSessionBearer(ctx, r, p)
}

func resolveCursorOAuthBearer(ctx context.Context, r *Root, p *Provider) (string, error) {
	if key := strings.TrimSpace(p.APIKey); key != "" {
		return key, nil
	}
	return resolveCursorSessionBearer(ctx, r, p)
}

func cursorSessionToken(p *Provider) string {
	if p == nil {
		return ""
	}
	for _, candidate := range []string{p.OAuthAccessToken, p.APIKey} {
		tok := strings.TrimSpace(candidate)
		if tok == "" || strings.HasPrefix(strings.ToLower(tok), "crsr_") {
			continue
		}
		return tok
	}
	return ""
}

func cursorRefreshSeed(p *Provider) string {
	if p == nil {
		return ""
	}
	if refresh := strings.TrimSpace(p.OAuthRefreshToken); refresh != "" {
		return refresh
	}
	return cursorSessionToken(p)
}

func cursorSessionNeedsRefresh(p *Provider, now time.Time) bool {
	tok := cursorSessionToken(p)
	if tok == "" {
		return true
	}
	exp, ok := p.oauthExpiresAt()
	if !ok {
		exp = cursorauth.AccessTokenExpiry(tok)
	}
	return now.Add(3 * time.Minute).After(exp)
}

func resolveCursorSessionBearer(ctx context.Context, r *Root, p *Provider) (string, error) {
	now := time.Now()
	stored := cursorSessionToken(p)
	// Cursor Sub owns its OAuth session. A local Cursor installation must not
	// supply credentials implicitly for the subscription provider.
	var desktop string
	if p != nil && !p.IsCursorSub() {
		desktop = cursorauth.DesktopAccessToken()
	}
	tok := stored
	if tok == "" {
		tok = desktop
	}
	if stored != "" && !cursorSessionNeedsRefresh(p, now) {
		return stored, nil
	}
	if stored == "" && desktop != "" {
		return desktop, nil
	}
	seed := cursorRefreshSeed(p)
	if seed == "" {
		logging.Log(logging.ERROR_LOG_LEVEL, "Cursor OAuth refresh token missing", logging.LogOptions{Params: map[string]any{"provider": p.Name}})
		return "", errors.New("Cursor: missing OAuth tokens; run /connect")
	}
	tokens, err := cursorauth.Refresh(ctx, seed)
	if err != nil {
		if tok != "" {
			logging.Log(logging.WARNING_LOG_LEVEL, "Cursor OAuth token refresh failed; using stored session", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
			return tok, nil
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "Cursor OAuth token refresh failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
		return "", fmt.Errorf("Cursor token refresh: %w", err)
	}
	ApplyCursorOAuthTokens(p, tokens)
	if r != nil {
		if err := Save(r); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "save config after Cursor OAuth refresh failed", logging.LogOptions{Params: map[string]any{"provider": p.Name, "err": err.Error()}})
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

func CursorSubChatBase() string {
	norm, err := NormalizeAPIBase(cursorauth.APIBase)
	if err != nil {
		return "https://api2.cursor.sh/v1/"
	}
	return norm
}

func EnsureCursorSubBaseURL(p *Provider) {
	if p == nil || !p.IsCursorSub() {
		return
	}
	p.BaseURL = CursorSubChatBase()
}

func NewClaudeSubProvider(tokens claude.TokenSet) (Provider, error) {
	norm, err := NormalizeAnthropicBase(AnthropicPlatformBase)
	if err != nil {
		return Provider{}, err
	}
	p := Provider{
		Name:        ProviderNameClaudeSub,
		BaseURL:     norm,
		APIProtocol: APIProtocolAnthropic,
	}
	ApplyClaudeOAuthTokens(&p, tokens)
	return p, nil
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
