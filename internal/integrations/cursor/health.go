package cursor

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const healthProtocol = 1

type healthIdentity struct {
	Protocol      int    `json:"protocol"`
	Bundle        string `json:"bundle"`
	CWD           string `json:"cwd"`
	InternalTools bool   `json:"internalTools"`
	Observability bool   `json:"observability"`
}

type healthResponse struct {
	OK       bool           `json:"ok"`
	Identity healthIdentity `json:"identity"`
	Proof    string         `json:"proof"`
}

func runtimeDigest(dir string) (string, error) {
	files := []string{"dist/index.js", "package.json", "package-lock.json"}
	entries, err := os.ReadDir(filepath.Join(dir, "dist", "prompts"))
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return "", fmt.Errorf("unexpected directory in sidecar prompts")
		}
		files = append(files, "dist/prompts/"+entry.Name())
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return "", err
		}
		_, _ = hash.Write([]byte(name + "\x00"))
		_, _ = hash.Write(data)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func expectedHealth(dir, cwd string, internal bool) (healthIdentity, error) {
	bundle, err := runtimeDigest(dir)
	if err != nil {
		return healthIdentity{}, fmt.Errorf("read cursor sidecar runtime identity: %w", err)
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return healthIdentity{}, err
	}
	return healthIdentity{healthProtocol, bundle, cwd, internal, sidecarProxyObsEnabled}, nil
}

func healthProof(key, nonce string, id healthIdentity) string {
	payload := strings.Join([]string{nonce, strconv.Itoa(id.Protocol), id.Bundle, id.CWD, strconv.FormatBool(id.InternalTools), strconv.FormatBool(id.Observability)}, "\n")
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifySidecar(ctx context.Context, port int, expected healthIdentity, key string) error {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return err
	}
	nonce := hex.EncodeToString(nonceBytes)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health?nonce=%s", port, nonce), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sidecar health request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sidecar health status %d", resp.StatusCode)
	}
	var health healthResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(&health); err != nil {
		return fmt.Errorf("invalid sidecar health metadata")
	}
	if !health.OK || health.Identity.Protocol != healthProtocol {
		return fmt.Errorf("sidecar health protocol mismatch; rebuild and restart the sidecar")
	}
	if health.Identity.Bundle != expected.Bundle {
		return fmt.Errorf("sidecar runtime bundle mismatch; rebuild and restart the sidecar")
	}
	if health.Identity.CWD != expected.CWD || health.Identity.InternalTools != expected.InternalTools || health.Identity.Observability != expected.Observability {
		return fmt.Errorf("sidecar configuration mismatch; restart the sidecar")
	}
	proof, err := hex.DecodeString(health.Proof)
	if err != nil {
		return fmt.Errorf("invalid sidecar health proof")
	}
	want, _ := hex.DecodeString(healthProof(key, nonce, health.Identity))
	if !hmac.Equal(proof, want) {
		return fmt.Errorf("sidecar credential proof mismatch; restart the sidecar with the current configuration")
	}
	return nil
}

func waitVerifiedHealth(ctx context.Context, port int, expected healthIdentity, key string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = verifySidecar(ctx, port, expected, key)
		if last == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return last
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
