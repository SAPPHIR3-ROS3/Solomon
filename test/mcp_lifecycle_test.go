package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCloseBeforeConnectUnblocksReadiness(t *testing.T) {
	m := solomonmcp.NewManagerForTest(nil, nil)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := m.WaitReady(ctx); !errors.Is(err, solomonmcp.ErrManagerClosed) {
		t.Fatalf("WaitReady = %v, want manager closed", err)
	}
	if _, _, err := m.Connect(ctx); !errors.Is(err, solomonmcp.ErrManagerClosed) {
		t.Fatalf("Connect = %v, want manager closed", err)
	}
}

func TestCloseDuringMCPStartup(t *testing.T) {
	logging.LogInit()
	if err := logging.Configure(logging.Config{WriteConsole: false, WriteFile: false}); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"server/discover", "tools/list"} {
		t.Run(phase, func(t *testing.T) {
			server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "lifecycle", Version: "1"}, nil)
			server.AddTool(&sdkmcp.Tool{Name: "ping", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{}, nil
			})
			delegate := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, &sdkmcp.StreamableHTTPOptions{Stateless: true})
			entered := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(body))
				var request struct {
					Method string `json:"method"`
				}
				_ = json.Unmarshal(body, &request)
				if request.Method == phase {
					once.Do(func() { close(entered) })
					select {
					case <-r.Context().Done():
						return
					case <-release:
					}
				}
				delegate.ServeHTTP(w, r)
			}))
			t.Cleanup(func() { close(release); httpServer.Close() })
			m := solomonmcp.NewManagerForTest(&solomonmcp.Config{Servers: []solomonmcp.ServerConfig{{Name: "first", Type: solomonmcp.TransportStreamableHTTP, URL: httpServer.URL}, {Name: "second", Type: solomonmcp.TransportStreamableHTTP, URL: httpServer.URL}}}, &solomonmcp.ManagerOptions{DisableCatalogSubscriptions: true})
			t.Cleanup(func() { _ = m.Close() })
			result := make(chan error, 1)
			go func() { _, _, err := m.Connect(context.Background()); result <- err }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("startup did not reach blocked phase")
			}
			session := m.FirstSessionForTest()
			if phase == "tools/list" && session == nil {
				t.Fatal("session must be tracked before catalog hydration")
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if !errors.Is(err, solomonmcp.ErrManagerClosed) {
					t.Fatalf("Connect = %v, want manager closed", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Close did not cancel pending startup")
			}
			if session != nil {
				stopped := make(chan struct{})
				go func() { _ = session.Wait(); close(stopped) }()
				select {
				case <-stopped:
				case <-time.After(time.Second):
					t.Fatal("Close leaked the session being hydrated")
				}
			}
			if count := m.ServerCountForTest(); count > 1 {
				t.Fatalf("shutdown connected another server: %d", count)
			}
		})
	}
}
