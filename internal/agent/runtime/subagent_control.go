package agentruntime

import (
	"context"
	"fmt"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func (r *Runtime) controlSubagent(id, action string) error {
	sess, err := chatstore.FindSubSessionByID(r.ProjHex, id)
	if err != nil {
		return err
	}
	if action == "resume" {
		if chatstore.SubSessionRunning(sess.Status) || globalSubagentRegistry.isRunning(id) {
			return fmt.Errorf("subagent %s is already running", id)
		}
		res, err := r.runSubagentTool(context.Background(), NestedRunConfig{
			ResumeID:         id,
			RunInBackground:  true,
			ParentChatID:     sess.ParentChatID,
			ParentToolCallID: sess.ParentToolCallID,
			ToolCall:         chatstore.ToolCall{ID: sess.ParentToolCallID, Name: "subagent"},
			SpawnTime:        time.Now().UTC(),
			Origin:           sess.Origin,
			ProjectHex:       sess.ProjectHex,
			SysPromptPath:    sess.SysPromptPath,
			RoleProvider:     sess.RoleProvider,
			RoleModel:        sess.RoleModel,
		})
		if err != nil {
			return err
		}
		commands.PrintSystemf(r.Out, "subagent %s resumed in background (%s)", sess.Title, res.SubchatID)
		return nil
	}
	if action != "stop" && action != "cancel" {
		return fmt.Errorf("unknown subagent control action: %s", action)
	}
	if err := globalSubagentRegistry.cancelAndWait(id, 10*time.Second); err != nil {
		return err
	}
	if action == "cancel" {
		sess.Status = chatstore.SubStatusCancelled
	} else {
		sess.Status = chatstore.SubStatusPaused
	}
	if err := chatstore.WriteSubSession(sess.ProjectHex, sess); err != nil {
		return err
	}
	return globalSubagentRegistry.removeActiveEntry(id)
}
