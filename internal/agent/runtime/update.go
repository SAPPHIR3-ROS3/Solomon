package agentruntime

import (
	"context"
	"errors"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func (r *Runtime) refreshUpdateCheck(ctx context.Context, force bool) (*updater.Notice, error) {
	r.updateMu.Lock()
	if !force && r.updateChecked {
		n := r.updateNotice
		err := r.updateCheckErr
		r.updateMu.Unlock()
		return n, err
	}
	r.updateMu.Unlock()

	res := updater.Check(ctx, commands.VersionString())
	var notice *updater.Notice
	if res.Err == nil && res.Newer {
		notice = res.Notice()
	}

	r.updateMu.Lock()
	r.updateChecked = true
	r.updateCheckErr = res.Err
	r.updateNotice = notice
	r.updateMu.Unlock()
	return notice, res.Err
}

func (r *Runtime) tryAutoUpdateInstall(ctx context.Context) (tag string, ok bool) {
	notice := r.cachedUpdateNotice()
	if notice == nil || r.Cfg == nil || !r.Cfg.AutoUpdateEnabled() {
		return "", false
	}
	err := updater.RunSystemInstall(ctx, notice.Latest, io.Discard)
	if errors.Is(err, updater.ErrRestartScheduled) {
		return notice.Latest, true
	}
	if err != nil {
		commands.PrintSystemErr(r.Out, err)
	}
	return "", false
}

func (r *Runtime) cachedUpdateNotice() *updater.Notice {
	r.updateMu.Lock()
	defer r.updateMu.Unlock()
	return r.updateNotice
}

func (r *Runtime) resetUpdateCache() {
	r.updateMu.Lock()
	r.updateChecked = false
	r.updateNotice = nil
	r.updateCheckErr = nil
	r.updateMu.Unlock()
}
