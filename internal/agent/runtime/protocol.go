package agentruntime

import (
	"context"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
)

func (r *Runtime) WaitReady(ctx context.Context) error { return r.waitProviderReady(ctx) }

func (r *Runtime) SnapshotSession() (*chatstore.Session, error) {
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	data, err := json.Marshal(r.Session)
	if err != nil {
		return nil, err
	}
	var sess chatstore.Session
	err = json.Unmarshal(data, &sess)
	return &sess, err
}

func (r *Runtime) SetMCPManager(manager *solomonmcp.Manager) {
	r.MCP = manager
	r.configureWebSearch(manager)
}
