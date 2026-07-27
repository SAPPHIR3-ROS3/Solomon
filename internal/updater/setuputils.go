package updater

import "context"

func UsesExecInstallRestartAfterSystemInstallForTest() bool {
	return UsesExecInstallRestartAfterSystemInstall()
}

func SimulateUpgradeExitRestartForTest(tag string) error {
	return FinishUpgradeRestart(context.Background(), tag)
}

func ExecInstallRestartForTest(ctx context.Context, tag string) error {
	return ExecInstallRestart(ctx, tag)
}

func WindowsInstallRestartScriptBodyForTest(pid int, tag, cwd, exe string, args []string) (string, error) {
	return windowsInstallRestartScriptBody(pid, tag, cwd, exe, args)
}
