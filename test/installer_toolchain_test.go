package test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUnixInstallerChecksLocalGoInsteadOfAutoDownloadedToolchain(t *testing.T) {
	for _, scenario := range []string{"older", "current", "forced"} {
		t.Run(scenario, func(t *testing.T) {
			command := exec.Command(installerBash(t), "-c", `
source "$1"
fixture_dir="$(cd "$2" && pwd)"; scenario="$3"
fixture_version=1.25.0; fixture_installs=0
[[ "$scenario" != current ]] || fixture_version="$GO_REQUIRED"
export GOTOOLCHAIN=auto
[[ "$scenario" != forced ]] || export GOTOOLCHAIN=go1.27.3
previous_toolchain="$GOTOOLCHAIN"
go() {
  case "$*" in
    version)
      if [[ "$GOTOOLCHAIN" == local ]]; then echo "go version go$fixture_version linux/amd64"
      else echo 'go version go1.99.0 linux/amd64'; fi ;;
    'env GOBIN') echo "$fixture_dir/bin" ;;
    *) echo "Unexpected go command: $*" >&2; return 95 ;;
  esac
}
install_go() { fixture_installs=$((fixture_installs + 1)); fixture_version="$GO_REQUIRED"; }
ensure_go
expected=1; [[ "$scenario" != current ]] || expected=0
[[ "$fixture_installs" == "$expected" ]]
[[ "$(go_semver)" == "$GO_REQUIRED" ]]
[[ "$GOTOOLCHAIN" == "$previous_toolchain" ]]
`, "installer-test", filepath.ToSlash(installerScriptFixture(t)), filepath.ToSlash(t.TempDir()), scenario)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("toolchain migration: %v\n%s", err, output)
			}
		})
	}
}

func TestPowerShellInstallerRestoresToolchainEnvironment(t *testing.T) {
	runWindowsInstallerFixture(t, `param($Installer, $Fixture)
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$definition = $ast.Find({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Get-GoSemVer'}, $true)
Invoke-Expression $definition.Extent.Text
function go {
 if ($env:GOTOOLCHAIN -ne 'local') { throw 'Automatic toolchain lookup was not disabled' }
 $global:LASTEXITCODE = 0
 if ($script:failure) { $global:LASTEXITCODE = 42 }
 'go version go1.25.0 windows/amd64'
}
foreach ($value in @($null, 'auto', 'go1.27.3')) {
 $env:GOTOOLCHAIN = $value
 $version = Get-GoSemVer
 if ($version -ne '1.25.0' -or $env:GOTOOLCHAIN -ne $value) { throw 'Incorrect toolchain version or leaked environment' }
 $script:failure = $true
 try { Get-GoSemVer; throw 'Accepted failed go command' } catch { if ($_ -notmatch 'go version failed') { throw } }
 if ($env:GOTOOLCHAIN -ne $value) { throw 'Toolchain environment not restored after failure' }
 $script:failure = $false
}
`)
}
