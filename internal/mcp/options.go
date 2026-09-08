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
}
