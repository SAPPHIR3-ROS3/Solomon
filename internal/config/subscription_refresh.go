package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

// Serialize reread -> refresh -> commit, not just the final file write. Refresh
// tokens rotate; two runtime snapshots must never spend the same grant twice.
var subscriptionRefreshMu sync.Mutex

// Only uncommitted rotations are retained, so a temporary disk error can be
// retried without spending the same refresh token a second time.
var pendingSubscriptionRotations = map[string]OAuthTokenSet{}

func resolveClaudeOAuthBearer(ctx context.Context, root *Root, provider *Provider) (string, error) {
	return resolveSubscriptionOAuthBearer(ctx, root, provider, AuthKindOAuthClaude, func(ctx context.Context, seed string) (OAuthTokenSet, error) {
		t, err := claude.Refresh(ctx, seed)
		return OAuthTokenSet{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresAt: t.ExpiresAt}, err
	})
}

func resolveChatGPTOAuthBearer(ctx context.Context, root *Root, provider *Provider) (string, error) {
	return resolveSubscriptionOAuthBearer(ctx, root, provider, AuthKindOAuthChatGPT, func(ctx context.Context, seed string) (OAuthTokenSet, error) {
		t, err := codex.Refresh(ctx, seed)
		return OAuthTokenSet{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresAt: t.ExpiresAt, AccountID: t.AccountID}, err
	})
}

func resolveSubscriptionOAuthBearer(ctx context.Context, root *Root, provider *Provider, kind string, refresh func(context.Context, string) (OAuthTokenSet, error)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	subscriptionRefreshMu.Lock()
	defer subscriptionRefreshMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !provider.oauthAccessExpired(time.Now()) && strings.TrimSpace(provider.OAuthAccessToken) != "" {
		return provider.OAuthAccessToken, nil
	}

	working := *provider
	var latest *Root
	if root != nil {
		path, err := paths.ConfigPath()
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", err
		}
		lock := flock.New(filepath.Join(filepath.Dir(path), "config.lock"))
		locked, err := lock.TryLockContext(ctx, 50*time.Millisecond)
		if err != nil {
			return "", fmt.Errorf("%s refresh lock: %w", provider.Name, err)
		}
		if !locked {
			return "", fmt.Errorf("%s refresh lock: %w", provider.Name, ctx.Err())
		}
		defer lock.Unlock()
		if _, err := os.Stat(path); err == nil {
			latest, err = Load()
			if err != nil {
				return "", err
			}
			current := ProviderByName(latest, provider.Name)
			if current == nil || current.EffectiveAuthKind() != kind {
				return "", fmt.Errorf("%s connection changed; run /connect", provider.Name)
			}
			working = *current
		} else if !os.IsNotExist(err) {
			return "", err
		} else {
			copyRoot := *root
			copyRoot.Providers = make(map[string]*Provider, len(root.Providers))
			for name, provider := range root.Providers {
				if provider == nil {
					continue
				}
				copyProvider := *provider
				copyRoot.Providers[name] = &copyProvider
			}
			latest = &copyRoot
		}
	}
	if !working.oauthAccessExpired(time.Now()) && strings.TrimSpace(working.OAuthAccessToken) != "" {
		updateSubscriptionSnapshot(root, provider, kind, OAuthTokenSet{AccessToken: working.OAuthAccessToken, RefreshToken: working.OAuthRefreshToken, ExpiresAt: parseSubscriptionExpiry(&working), AccountID: working.OAuthAccountID})
		return provider.OAuthAccessToken, nil
	}
	if strings.TrimSpace(working.OAuthRefreshToken) == "" {
		return "", fmt.Errorf("%s: missing OAuth tokens; run /connect", provider.Name)
	}
	rotationKey := kind + "\x00" + working.OAuthRefreshToken
	tokens, pending := pendingSubscriptionRotations[rotationKey]
	if !pending {
		var err error
		tokens, err = refresh(ctx, working.OAuthRefreshToken)
		if err != nil {
			return "", fmt.Errorf("%s token refresh: %w", provider.Name, err)
		}
		if latest != nil {
			pendingSubscriptionRotations[rotationKey] = tokens
		}
	}
	if tokens.AccountID == "" {
		tokens.AccountID = working.OAuthAccountID
	}
	// Commit before making this pair available to inference.
	if latest != nil {
		target := ProviderByName(latest, provider.Name)
		if target == nil {
			return "", fmt.Errorf("%s provider missing while saving refreshed tokens", provider.Name)
		}
		applyOAuthTokens(target, kind, tokens)
		if err := saveUnlocked(latest); err != nil {
			return "", fmt.Errorf("save rotated %s credentials: %w", provider.Name, err)
		}
	}
	delete(pendingSubscriptionRotations, rotationKey)
	updateSubscriptionSnapshot(root, provider, kind, tokens)
	return tokens.AccessToken, nil
}

func parseSubscriptionExpiry(provider *Provider) time.Time {
	expiry, _ := provider.oauthExpiresAt()
	return expiry
}

func updateSubscriptionSnapshot(root *Root, provider *Provider, kind string, tokens OAuthTokenSet) {
	applyOAuthTokens(provider, kind, tokens)
	if root != nil {
		if target := ProviderByName(root, provider.Name); target != nil && target != provider {
			applyOAuthTokens(target, kind, tokens)
		}
	}
}
