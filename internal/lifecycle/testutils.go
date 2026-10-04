package lifecycle

import "context"

// UpdateOpsForTest lets the repository's top-level tests exercise recovery
// without touching any running application or external release.
type UpdateOpsForTest struct {
	Prepare func() (string, string, error)
	Stop    func(string) error
	Commit  func(string, string) error
	Start   func(string) error
	Restore func(string) error
	Quiesce func() error
}

func ApplyUpdateForTest(ctx context.Context, tag string, ops UpdateOpsForTest) error {
	return applyRuntimeUpdate(ctx, tag, runtimeUpdateOps{prepare: ops.Prepare, stop: ops.Stop, commit: ops.Commit, start: ops.Start, restore: ops.Restore, quiesce: ops.Quiesce})
}

type DesktopProcessForTest struct {
	PID      int
	Identity string
}

func DesktopProcessesForTest(executable string) ([]DesktopProcessForTest, error) {
	processes, err := installedDesktopProcesses(executable)
	var result []DesktopProcessForTest
	for _, process := range processes {
		result = append(result, DesktopProcessForTest{PID: process.pid, Identity: process.identity})
	}
	return result, err
}

func ProcessIdentityForTest(pid int) string { return processIdentity(pid) }
func WaitProcessExitForTest(ctx context.Context, pid int, identity string) error {
	return waitProcessExit(ctx, pid, identity)
}

func UpdateClientsForTest(executable string) ([]DesktopProcessForTest, error) {
	clients, err := updateClients(executable)
	var result []DesktopProcessForTest
	for _, p := range clients {
		result = append(result, DesktopProcessForTest{PID: p.pid, Identity: p.identity})
	}
	return result, err
}
func StopClientForTest(executable string, pid int) error {
	clients, err := updateClients(executable)
	if err != nil {
		return err
	}
	for _, p := range clients {
		if p.pid == pid {
			return requestClientStop(p)
		}
	}
	return nil
}
