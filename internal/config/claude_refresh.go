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
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

// Serialize reread -> refresh -> commit, not just the final file write. Refresh
// tokens rotate; two runtime snapshots must never spend the same grant twice.
var claudeRefreshMu sync.Mutex

// Only uncommitted rotations are retained, so a temporary disk error can be
// retried without spending the same refresh token a second time.
var pendingClaudeRotations = map[string]claude.TokenSet{}

func resolveClaudeOAuthBearer(ctx context.Context, root *Root, provider *Provider) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	claudeRefreshMu.Lock()
	defer claudeRefreshMu.Unlock()
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
			return "", fmt.Errorf("Claude Sub refresh lock: %w", err)
		}
		if !locked {
			return "", fmt.Errorf("Claude Sub refresh lock: %w", ctx.Err())
		}
		defer lock.Unlock()
		if _, err := os.Stat(path); err == nil {
			latest, err = Load()
			if err != nil {
				return "", err
			}
			current := ProviderByName(latest, provider.Name)
			if current == nil || !current.IsClaudeSub() {
				return "", fmt.Errorf("Claude Sub connection changed; run /connect")
			}
			working = *current
		} else if !os.IsNotExist(err) {
			return "", err
		} else {
			copyRoot := *root
			copyRoot.Providers = make(map[string]*Provider, len(root.Providers))
			for name, provider := range root.Providers {
				copyProvider := *provider
				copyRoot.Providers[name] = &copyProvider
			}
			latest = &copyRoot
		}
	}
	if !working.oauthAccessExpired(time.Now()) && strings.TrimSpace(working.OAuthAccessToken) != "" {
		ApplyClaudeOAuthTokens(provider, claude.TokenSet{AccessToken: working.OAuthAccessToken, RefreshToken: working.OAuthRefreshToken, ExpiresAt: parseClaudeExpiry(&working)})
		return provider.OAuthAccessToken, nil
	}
	if strings.TrimSpace(working.OAuthRefreshToken) == "" {
		return "", fmt.Errorf("Claude Sub: missing OAuth tokens; run /connect")
	}
	tokens, pending := pendingClaudeRotations[working.OAuthRefreshToken]
	if !pending {
		var err error
		tokens, err = claude.Refresh(ctx, working.OAuthRefreshToken)
		if err != nil {
			return "", fmt.Errorf("Claude Sub token refresh: %w", err)
		}
		if latest != nil {
			pendingClaudeRotations[working.OAuthRefreshToken] = tokens
		}
	}
	// Commit before making this pair available to inference.
	if latest != nil {
		target := ProviderByName(latest, provider.Name)
		if target == nil {
			return "", fmt.Errorf("Claude Sub provider missing while saving refreshed tokens")
		}
		ApplyClaudeOAuthTokens(target, tokens)
		if err := saveUnlocked(latest); err != nil {
			return "", fmt.Errorf("save rotated Claude Sub credentials: %w", err)
		}
	}
	delete(pendingClaudeRotations, working.OAuthRefreshToken)
	ApplyClaudeOAuthTokens(provider, tokens)
	return tokens.AccessToken, nil
}

func parseClaudeExpiry(provider *Provider) time.Time {
	expiry, _ := provider.oauthExpiresAt()
	return expiry
}
