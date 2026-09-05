package codex

var (
	ChatGPTSubAPIBase   = "https://chatgpt.com/backend-api/codex"
	ChatGPTResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
)

const (
	AuthorizeURL = "https://auth.openai.com/oauth/authorize"
	TokenURL     = "https://auth.openai.com/oauth/token"
	ClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	RedirectURI  = "http://localhost:1455/auth/callback"
	Scopes       = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	Originator   = "codex_cli_rs"
	// ClientVersion is the offline fallback; requests resolve npm latest dynamically.
	ClientVersion = "0.153.3"
	CallbackAddr  = "127.0.0.1:1455"
	CallbackPath  = "/auth/callback"
)
