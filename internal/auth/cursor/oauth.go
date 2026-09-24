package cursor

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	loginURL            = "https://cursor.com/loginDeepControl"
	pollURL             = "https://api2.cursor.sh/auth/poll"
	refreshURL          = "https://api2.cursor.sh/auth/exchange_user_api_key"
	APIBase             = "https://api2.cursor.sh"
	cursorClientVersion = "3.21.13"
	cursorClientCommit  = "e44a49c17e334d442e58bbde931d791200f014a0"

	pollMaxAttempts      = 150
	pollMaxDelay         = 10 * time.Second
	maxConsecutiveErrors = 10
)

var (
	pollEndpoint    = pollURL
	refreshEndpoint = refreshURL
	pollBaseDelay   = time.Second
)

type AuthParams struct {
	Verifier  string
	Challenge string
	UUID      string
	LoginURL  string
}

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	APIKey       string
	ExpiresAt    time.Time
}

type tokenPair struct {
	AccessToken     string `json:"accessToken"`
	RefreshToken    string `json:"refreshToken"`
	AccessTokenAlt  string `json:"access_token"`
	RefreshTokenAlt string `json:"refresh_token"`
}

func GenerateAuthParams() (AuthParams, error) {
	verifierBytes := make([]byte, 96)
	if _, err := rand.Read(verifierBytes); err != nil {
		return AuthParams{}, fmt.Errorf("cursor login: pkce verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	id, err := randomUUID()
	if err != nil {
		return AuthParams{}, err
	}
	q := url.Values{}
	q.Set("challenge", challenge)
	q.Set("uuid", id)
	q.Set("mode", "login")
	q.Set("redirectTarget", "cli")
	return AuthParams{
		Verifier:  verifier,
		Challenge: challenge,
		UUID:      id,
		LoginURL:  loginURL + "?" + q.Encode(),
	}, nil
}

func PollForAuth(ctx context.Context, uuid, verifier string) (TokenSet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	delay := pollBaseDelay
	consecutiveErrors := 0
	client := &http.Client{Timeout: 10 * time.Second}
	for attempt := 0; attempt < pollMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return TokenSet{}, ctx.Err()
			case <-time.After(delay):
			}
		}
		status, body, err := pollOnce(ctx, client, uuid, verifier)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				return TokenSet{}, fmt.Errorf("cursor login: poll failed: %w", err)
			}
			delay = nextPollDelay(delay)
			continue
		}
		if status == http.StatusNotFound || status == http.StatusNoContent {
			consecutiveErrors = 0
			delay = nextPollDelay(delay)
			continue
		}
		if status < 200 || status >= 300 {
			return TokenSet{}, fmt.Errorf("cursor login: poll status %d: %s", status, strings.TrimSpace(string(body)))
		}
		tokens, parseErr := tokenSetFromPair(body, "")
		if parseErr != nil {
			consecutiveErrors = 0
			delay = nextPollDelay(delay)
			continue
		}
		return tokens, nil
	}
	return TokenSet{}, fmt.Errorf("cursor login: timed out waiting for browser sign-in")
}

func pollOnce(ctx context.Context, client *http.Client, uuid, verifier string) (int, []byte, error) {
	status, body, err := doPoll(ctx, client, http.MethodPost, uuid, verifier)
	if err != nil {
		return 0, nil, err
	}
	if status != http.StatusNotFound {
		return status, body, nil
	}
	return doPoll(ctx, client, http.MethodGet, uuid, verifier)
}

func doPoll(ctx context.Context, client *http.Client, method, uuid, verifier string) (int, []byte, error) {
	rawURL := pollEndpoint
	var body io.Reader
	if method == http.MethodGet {
		q := url.Values{}
		q.Set("uuid", uuid)
		q.Set("verifier", verifier)
		rawURL += "?" + q.Encode()
	} else {
		payload, err := json.Marshal(map[string]string{"uuid": uuid, "verifier": verifier})
		if err != nil {
			return 0, nil, err
		}
		body = strings.NewReader(string(payload))
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://cursor.com")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	return resp.StatusCode, raw, nil
}

func Refresh(ctx context.Context, refreshToken string) (TokenSet, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenSet{}, fmt.Errorf("cursor login: missing refresh token")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshEndpoint, strings.NewReader("{}"))
	if err != nil {
		return TokenSet{}, err
	}
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenSet{}, fmt.Errorf("cursor login: refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenSet{}, fmt.Errorf("cursor login: refresh status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return tokenSetFromPair(body, refreshToken)
}

func tokenSetFromPair(body []byte, fallbackRefresh string) (TokenSet, error) {
	var pair tokenPair
	if err := json.Unmarshal(body, &pair); err != nil {
		return TokenSet{}, fmt.Errorf("cursor login: parse tokens: %w", err)
	}
	access := firstNonEmpty(pair.AccessToken, pair.AccessTokenAlt)
	if access == "" {
		return TokenSet{}, fmt.Errorf("cursor login: missing access token")
	}
	refresh := firstNonEmpty(pair.RefreshToken, pair.RefreshTokenAlt, fallbackRefresh)
	return TokenSet{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiryFromAccessToken(access),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func randomUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("cursor login: uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func DesktopAccessToken() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(configDir, "Cursor", "auth.json"))
	if err != nil {
		return ""
	}
	var auth struct {
		AccessToken string `json:"accessToken"`
	}
	if json.Unmarshal(raw, &auth) != nil {
		return ""
	}
	return strings.TrimSpace(auth.AccessToken)
}

func AccessTokenExpiry(token string) time.Time {
	return expiryFromAccessToken(token)
}

func expiryFromAccessToken(token string) time.Time {
	fallback := time.Now().Add(time.Hour)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fallback
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(padBase64(parts[1]))
		if err != nil {
			return fallback
		}
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp <= 0 {
		return fallback
	}
	exp := time.Unix(int64(claims.Exp), 0).Add(-5 * time.Minute)
	if exp.Before(time.Now()) {
		return time.Now().Add(time.Minute)
	}
	return exp
}

func padBase64(value string) string {
	switch len(value) % 4 {
	case 2:
		return value + "=="
	case 3:
		return value + "="
	default:
		return value
	}
}

func cursorClientVersionHeader() string {
	if v := strings.TrimSpace(os.Getenv("CURSOR_CLIENT_VERSION")); v != "" {
		return v
	}
	for _, path := range cursorProductJSONPaths() {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var product struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		}
		if json.Unmarshal(raw, &product) != nil {
			continue
		}
		if strings.TrimSpace(product.Version) != "" {
			return strings.TrimSpace(product.Version)
		}
	}
	return cursorClientVersion
}

func cursorClientCommitHeader() string {
	if v := strings.TrimSpace(os.Getenv("CURSOR_CLIENT_COMMIT")); v != "" {
		return v
	}
	for _, path := range cursorProductJSONPaths() {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var product struct {
			Commit string `json:"commit"`
		}
		if json.Unmarshal(raw, &product) != nil {
			continue
		}
		if strings.TrimSpace(product.Commit) != "" {
			return strings.TrimSpace(product.Commit)
		}
	}
	return cursorClientCommit
}

func cursorProductJSONPaths() []string {
	var paths []string
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		paths = append(paths, filepath.Join(local, "Programs", "cursor", "resources", "app", "product.json"))
	}
	if pf := strings.TrimSpace(os.Getenv("ProgramFiles")); pf != "" {
		paths = append(paths, filepath.Join(pf, "cursor", "resources", "app", "product.json"))
	}
	paths = append(paths,
		filepath.Join("/Applications", "Cursor.app", "Contents", "Resources", "app", "product.json"),
		filepath.Join("/usr", "share", "cursor", "resources", "app", "product.json"),
	)
	return paths
}

func cursorClientOS() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	case "darwin":
		return "darwin"
	default:
		return "linux"
	}
}

func cursorClientArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

func cursorClientOSVersion() string {
	if runtime.GOOS == "windows" {
		return "10.0.26200"
	}
	return runtime.GOOS
}

func hashed64Hex(s, salt string) string {
	sum := sha256.Sum256([]byte(s + salt))
	return hex.EncodeToString(sum[:])
}

func uuidV5DNS(name string) string {
	ns := []byte{0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1, 0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8}
	h := sha1.New()
	h.Write(ns)
	h.Write([]byte(name))
	sum := h.Sum(nil)[:16]
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return hex.EncodeToString(sum[0:4]) + "-" + hex.EncodeToString(sum[4:6]) + "-" + hex.EncodeToString(sum[6:8]) + "-" + hex.EncodeToString(sum[8:10]) + "-" + hex.EncodeToString(sum[10:16])
}

func cursorChecksum(token string) string {
	ts := time.Now().UnixMilli() / 1_000_000
	b := []byte{byte(ts >> 40), byte(ts >> 32), byte(ts >> 24), byte(ts >> 16), byte(ts >> 8), byte(ts)}
	t := byte(165)
	for i := range b {
		b[i] = (b[i] ^ t) + byte(i)
		t = b[i]
	}
	machineID, macMachineID := cursorMachineIDs()
	if machineID == "" {
		machineID = hashed64Hex(token, "machineId")
	}
	if macMachineID == "" {
		macMachineID = hashed64Hex(token, "macMachineId")
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(b), "=") + machineID + "/" + macMachineID
}

func cursorMachineIDs() (machineID, macMachineID string) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", ""
	}
	paths := []string{
		filepath.Join(configDir, "Cursor", "User", "globalStorage", "storage.json"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "Cursor", "User", "globalStorage", "storage.json"))
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var storage map[string]any
		if json.Unmarshal(raw, &storage) != nil {
			continue
		}
		machineID, _ = storage["telemetry.machineId"].(string)
		macMachineID, _ = storage["telemetry.macMachineId"].(string)
		if strings.TrimSpace(machineID) != "" && strings.TrimSpace(macMachineID) != "" {
			return strings.TrimSpace(machineID), strings.TrimSpace(macMachineID)
		}
	}
	return "", ""
}

func ResolveChatModelID(id, effort string, fast bool) string {
	base := CanonicalCursorModelID(cursorBaseModelID(id))
	if base == "" {
		base = "cursor-composer-2.5"
	}
	level := cursorEffortSuffix(effort)
	synth := base
	if level != "" {
		synth += "-" + level
	}
	if fast {
		synth += "-fast"
	}
	fastMu.Lock()
	variants := append([]string(nil), knownVariants...)
	fastMu.Unlock()
	for _, raw := range variants {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.EqualFold(CanonicalCursorModelID(raw), synth) {
			return CanonicalCursorModelID(raw)
		}
	}
	return synth
}

func cursorEffortSuffix(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "", "none", "off":
		return ""
	case "med", "mid":
		return "medium"
	case "x-high", "extra-high", "extra_high":
		return "xhigh"
	case "low", "medium", "high", "xhigh", "max":
		return strings.ToLower(strings.TrimSpace(effort))
	default:
		return strings.ToLower(strings.TrimSpace(effort))
	}
}

func nextPollDelay(delay time.Duration) time.Duration {
	next := delay * 6 / 5
	if next < time.Second {
		next = time.Second
	}
	if next > pollMaxDelay {
		return pollMaxDelay
	}
	return next
}
