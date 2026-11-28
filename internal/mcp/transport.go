package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	providertransport "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/weblimit"
)

type commandTransport struct {
	command string
	args    []string
	cwd     string
	env     map[string]string
	stderr  io.Writer
}

func (t *commandTransport) Connect(ctx context.Context) (sdkmcp.Connection, error) {
	cmd := exec.Command(t.command, t.args...)
	if t.cwd != "" {
		cmd.Dir = t.cwd
	}
	cmd.Env = os.Environ()
	for k, v := range t.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio stdin pipe failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio stdout pipe failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		return nil, err
	}
	if t.stderr != nil {
		cmd.Stderr = t.stderr
	}
	if err := cmd.Start(); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio process start failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		return nil, err
	}
	conn, err := (&sdkmcp.IOTransport{Reader: stdout, Writer: stdin}).Connect(ctx)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP stdio IO transport connect failed", logging.LogOptions{Params: map[string]any{"command": t.command, "err": err.Error()}})
		_ = cmd.Process.Kill()
		started := time.Now()
		waitErr := cmd.Wait()
		logging.ProcessFinished(ctx, cmd, "mcp.stdio.wait", started, waitErr)
		return nil, err
	}
	waitDone := make(chan struct{})
	go func() {
		started := time.Now()
		waitErr := cmd.Wait()
		logging.ProcessFinished(ctx, cmd, "mcp.stdio.wait", started, waitErr)
		close(waitDone)
	}()
	return &processConnection{Connection: conn, cmd: cmd, waitDone: waitDone}, nil
}

type processConnection struct {
	sdkmcp.Connection
	cmd      *exec.Cmd
	waitDone chan struct{}
}

func (c *processConnection) Close() error {
	err := c.Connection.Close()
	if c.cmd.Process != nil {
		logging.ProcessTermination(c.cmd.Process.Pid, "mcp.stdio", "connection_closed")
		_ = c.cmd.Process.Kill()
	}
	<-c.waitDone
	return err
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
	manager *Manager
	rateKey string
}

// persistentSSETransport keeps the initial connection timeout from cancelling
// the long-lived SSE stream once the MCP session has been established. The
// SDK's SSE transport binds the stream to the context passed to Connect, while
// Manager.connectServer quite deliberately cancels its setup context after
// catalog discovery.
type persistentSSETransport struct {
	transport sdkmcp.SSEClientTransport
}

func (t *persistentSSETransport) Connect(ctx context.Context) (sdkmcp.Connection, error) {
	transportCtx, cancel := context.WithCancel(context.Background())
	relayDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-relayDone:
		}
	}()
	conn, err := t.transport.Connect(transportCtx)
	close(relayDone)
	if err != nil {
		cancel()
		return nil, err
	}
	return conn, nil
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.manager != nil {
		if err := t.manager.checkInternalRateLimit(req.Context(), t.rateKey); err != nil {
			return nil, err
		}
	}
	clone := req.Clone(req.Context())
	for k, v := range t.headers {
		clone.Header.Set(k, v)
	}
	response, err := t.base.RoundTrip(clone)
	if err != nil || t.rateKey == "" || response.StatusCode != http.StatusTooManyRequests {
		return response, err
	}
	defer response.Body.Close()
	var message string
	if strings.Contains(response.Header.Get("Content-Type"), "json") {
		body, err := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		if err == nil && json.Valid(body) {
			message = string(body)
		}
	}
	failure := providertransport.NewProviderHTTPError(http.StatusTooManyRequests, message, providertransport.ParseRetryAfterHeader(response))
	reset, _ := weblimit.Reset(failure, time.Now())
	t.manager.recordInternalRateLimit(req.Context(), t.rateKey, reset)
	return nil, &weblimit.Recorded{Err: failure}
}

func (m *Manager) httpClient(sc ServerConfig) *http.Client {
	base := networkhttp.New(http.DefaultTransport.(*http.Transport))
	return &http.Client{Transport: &headerTransport{
		base: base, headers: sc.Headers, manager: m, rateKey: internalRateLimitKey(sc),
	}}
}

func (m *Manager) transportFor(ctx context.Context, sc ServerConfig, stderr io.Writer) (sdkmcp.Transport, error) {
	if sc.Type == TransportStreamableHTTP {
		oauthHandler, err := m.oauthHandlerFor(ctx, sc)
		if err != nil {
			return nil, err
		}
		transport := &sdkmcp.StreamableClientTransport{
			Endpoint:   sc.URL,
			HTTPClient: m.httpClient(sc),
			MaxRetries: 5,
		}
		if oauthHandler != nil {
			transport.OAuthHandler = oauthHandler
		}
		return transport, nil
	}
	if sc.Type == TransportSSE {
		return &persistentSSETransport{
			transport: sdkmcp.SSEClientTransport{
				Endpoint:   sc.URL,
				HTTPClient: m.httpClient(sc),
			},
		}, nil
	}
	return &commandTransport{
		command: sc.Command,
		args:    sc.Args,
		cwd:     sc.CWD,
		env:     sc.Env,
		stderr:  stderr,
	}, nil
}

// oauthHandlerFor returns one handler per configured server. The handler owns
// the token source, including refresh state, so reconnecting an MCP session
// does not restart the OAuth flow. Persistence across Solomon processes stays
// with the host-provided handler/factory and is never written to mcp.json.
func (m *Manager) oauthHandlerFor(ctx context.Context, sc ServerConfig) (auth.OAuthHandler, error) {
	if m == nil || (m.options.OAuthHandler == nil && sc.OAuth == nil) {
		return nil, nil
	}
	m.oauthMu.Lock()
	defer m.oauthMu.Unlock()
	if m.oauthReady[sc.Name] {
		return m.oauthHandlers[sc.Name], nil
	}
	var (
		h   auth.OAuthHandler
		err error
	)
	if m.options.OAuthHandler != nil {
		h, err = m.options.OAuthHandler(ctx, sc)
	} else {
		h, err = m.oauthHandler(ctx, sc)
	}
	if err != nil {
		return nil, err
	}
	m.oauthHandlers[sc.Name] = h
	m.oauthReady[sc.Name] = true
	return h, nil
}

func (m *Manager) oauthHandler(_ context.Context, sc ServerConfig) (auth.OAuthHandler, error) {
	if sc.OAuth == nil {
		return nil, nil
	}
	if m.options.AuthorizationCodeFetcher == nil {
		return nil, fmt.Errorf("MCP server %q declares OAuth but no AuthorizationCodeFetcher was configured", sc.Name)
	}
	oauthCfg := sc.OAuth
	handlerCfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              oauthCfg.RedirectURL,
		AuthorizationCodeFetcher: m.options.AuthorizationCodeFetcher,
		RequestRefreshToken:      oauthCfg.RequestRefresh,
		Client:                   m.options.OAuthHTTPClient,
	}
	if oauthCfg.ClientIDMetadataURL != "" {
		handlerCfg.ClientIDMetadataDocumentConfig = &auth.ClientIDMetadataDocumentConfig{URL: oauthCfg.ClientIDMetadataURL}
	}
	if oauthCfg.ClientID != "" {
		handlerCfg.PreregisteredClient = &oauthex.ClientCredentials{ClientID: oauthCfg.ClientID, ClientSecretAuth: nil}
		if oauthCfg.ClientSecret != "" {
			handlerCfg.PreregisteredClient.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: oauthCfg.ClientSecret}
		}
	}
	if len(oauthCfg.RedirectURIs) > 0 {
		handlerCfg.DynamicClientRegistrationConfig = &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				RedirectURIs: oauthCfg.RedirectURIs,
				ClientName:   oauthCfg.ClientName,
				ClientURI:    oauthCfg.ClientURI,
				Scope:        oauthCfg.Scope,
			},
		}
	}
	return auth.NewAuthorizationCodeHandler(handlerCfg)
}
