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
	ClientVersion = "0.144.1"
	UserAgent    = "codex_cli_rs/" + ClientVersion + " (Ubuntu 22.04.0; x86_64) WindowsTerminal"
	CallbackAddr = "127.0.0.1:1455"
	CallbackPath = "/auth/callback"
)
