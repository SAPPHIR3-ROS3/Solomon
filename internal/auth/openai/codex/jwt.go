package codex

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func ChatGPTAccountIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	auth, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return ""
	}
	id, ok := auth["chatgpt_account_id"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(id)
}
