package mcp

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ManagerOptions configures the MCP host capabilities exposed by Solomon.
//
// ClientOptions is passed to every MCP client. It can provide handlers for
// elicitation, sampling, progress, logging, list changes, and resource
// updates. The manager wraps the list-change handlers so its own catalogs are
// refreshed asynchronously without blocking the SDK's notification reader.
type ManagerOptions struct {
	ClientOptions sdkmcp.ClientOptions

	// Roots are advertised to MCP servers that request roots/list. They can be
	// changed later with AddRoots and RemoveRoots.
	Roots []*sdkmcp.Root

	// OAuthHandler creates the OAuth handler for a configured HTTP server. The
	// factory is intentionally injectable: interactive authorization and token
	// persistence belong to the host application, not to mcp.json.
	OAuthHandler func(context.Context, ServerConfig) (auth.OAuthHandler, error)

	// AuthorizationCodeFetcher starts an interactive OAuth authorization-code
	// flow when a server config declares OAuth but no custom OAuthHandler
	// factory is supplied.
	AuthorizationCodeFetcher auth.AuthorizationCodeFetcher
	OAuthHTTPClient          *http.Client

	// DisableCatalogSubscriptions disables the modern subscriptions/listen
	// stream used only for catalog-change notifications. Catalogs are still
	// refreshed after reconnect and explicit list operations; this is useful
	// for stateless/internal adapters that only need request-response calls.
	DisableCatalogSubscriptions bool

	// RetryToolCall may explicitly authorize replaying a tool call after a
	// connection failure. By default the manager only retries calls rejected
	// with ErrSessionMissing, because MCP tool annotations are advisory and a
	// lost response can otherwise duplicate side effects.
	RetryToolCall func(serverName, toolName string, definition *sdkmcp.Tool, cause error) bool
}
