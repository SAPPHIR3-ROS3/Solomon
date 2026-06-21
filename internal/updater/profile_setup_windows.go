//go:build windows

package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

func runWindowsProfileSetup(ctx context.Context, progress io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	resp, err := httpDownload(ctx, installPS1RawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download install.ps1: %s", resp.Status)
	}
	tmp, err := os.CreateTemp("", "solomon-install-*.ps1")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	ok = true
	cmd := exec.CommandContext(ctx, windowsPowerShellExe(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpPath, "-ProfileOnly")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if progress != nil {
		fmt.Fprintln(progress, "PowerShell profile updated.")
	}
	return nil
}

func windowsProfileSetupScriptLines() string {
	return fmt.Sprintf(`Write-Host 'Updating PowerShell profile...'
$setupScript = [System.IO.Path]::ChangeExtension([System.IO.Path]::GetTempFileName(), '.ps1')
try {
  Invoke-WebRequest -Uri '%s' -OutFile $setupScript -UseBasicParsing
  & $setupScript -ProfileOnly
} catch {
  Write-Warning "PowerShell profile update failed: $_"
} finally {
  Remove-Item -Force $setupScript -ErrorAction SilentlyContinue
}
`, psQuote(installPS1RawURL))
}
