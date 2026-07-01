package updater

import "context"

func UsesExecInstallRestartAfterSystemInstall() bool {
	return canExecInstallRestart()
}

func FinishUpgradeRestart(ctx context.Context, tag string) error {
	if !UsesExecInstallRestartAfterSystemInstall() {
		return nil
	}
	return ExecInstallRestart(ctx, tag)
}
