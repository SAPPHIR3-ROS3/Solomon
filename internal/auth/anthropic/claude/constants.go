package claude

const (
	AuthorizeURL = "https://claude.ai/oauth/authorize"
	TokenURL     = "https://platform.claude.com/v1/oauth/token"
	ClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	Scopes       = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	CallbackHost = "127.0.0.1"
	CallbackPort = 53692
	CallbackPath = "/callback"
	RedirectURI  = "http://localhost:53692/callback"
	CallbackAddr = "127.0.0.1:53692"
)
