package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func (a *modelAPI) handleQuotaReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var input struct {
		Provider       string `json:"provider"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if err := decodeJSONBody(w, r, &input, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.IdempotencyKey) == "" || len(input.IdempotencyKey) > 128 {
		writeAPIError(w, http.StatusBadRequest, errors.New("provider and idempotency key are required"))
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	for _, provider := range config.ProviderList(cfg) {
		if provider.Name != input.Provider {
			continue
		}
		if !provider.IsChatGPTSub() {
			writeAPIError(w, http.StatusBadRequest, errors.New("banked resets are only available for ChatGPT subscriptions"))
			return
		}
		bearer, err := config.ResolveProviderBearer(r.Context(), cfg, &provider)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, err)
			return
		}
		code, err := consumeChatGPTReset(r.Context(), bearer, provider.OAuthAccountID, input.IdempotencyKey)
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"code": code})
		return
	}
	writeAPIError(w, http.StatusNotFound, errors.New("provider not found"))
}

func consumeChatGPTReset(ctx context.Context, bearer, accountID, key string) (string, error) {
	return consumeChatGPTResetWithClient(ctx, bearer, accountID, key, providerQuotaHTTPClient)
}

func consumeChatGPTResetWithClient(ctx context.Context, bearer, accountID, key string, client *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]string{"redeem_request_id": key})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("ChatGPT-Account-Id", accountID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Originator", codex.Originator)
	req.Header.Set("User-Agent", quotaUserAgent("chatgpt"))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ChatGPT reset: %s", quotaErrorFromBody(resp.Status, data))
	}
	var result struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	switch result.Code {
	case "reset", "already_redeemed", "nothing_to_reset", "no_credit":
		return result.Code, nil
	default:
		return "", errors.New("unexpected ChatGPT reset response")
	}
}

func ConsumeChatGPTResetForTest(ctx context.Context, bearer, accountID, key string, client *http.Client) (string, error) {
	return consumeChatGPTResetWithClient(ctx, bearer, accountID, key, client)
}
