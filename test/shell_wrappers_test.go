package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/shellpolicy"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/validation"
)

func TestShellWrapperInlineProgramsAreRejected(t *testing.T) {
	for _, command := range []string{
		`$(printf node) -e code`,
		`printf "%s" "$((1 + $(node -e code)))"`,
		`printf "%s" "$(node -e code)"`,
		`xargs -0 -n 1 node -e 'console.log(1)'`,
		`find . -exec python3 -c 'print(1)' \;`,
		`find . -execdir node -e 'console.log(1)' {} +`,
		`docker exec -it container node -e 'console.log(1)'`,
		`docker run --rm image python3 -c 'print(1)'`,
		`docker run --entrypoint python3 image -c 'print(1)'`,
		`docker compose exec service node -e 'console.log(1)'`,
		`podman exec container ruby -e 'puts 1'`,
		`kubectl -n ns exec pod -- node -e 'console.log(1)'`,
		`env -i -S "python3 -c 'print(1)'"`,
		`env --split-string="node -e 'console.log(1)'"`,
		`2>&1 python3 -c 'print(1)'`,
		`> /tmp/log node -e 'console.log(1)'`,
		`<>/tmp/log node -e 'console.log(1)'`,
		`alias p=python3; p -c 'print(1)'`,
		`source /tmp/helper.sh`,
		`node --import=data:text/javascript,console.log(1) file.js`,
		`node -rdata:text/javascript,console.log(1) file.js`,
		`python3 -W ignore -c 'print(1)'`,
		`ssh -o "ProxyCommand=python3 -c 'print(1)'" host true`,
	} {
		t.Run(command, func(t *testing.T) {
			err := shellpolicy.Validate(command)
			if validation.Kind(err) != "policy" {
				t.Fatalf("inline program accepted or unclassified: %s: %v", command, err)
			}
		})
	}
}

func TestShellExistingRunnerArgumentsRemainData(t *testing.T) {
	for _, command := range []string{
		`node scripts/check.js -e expected`,
		`n=1; n=$((n+1)); printf "%s" "$n"`,
		`python3 scripts/check.py -c config.toml`,
		`node --require ./preload.js scripts/check.js`,
		`node -r./preload.js scripts/check.js`,
		`python3 -W ignore scripts/check.py`,
		`python3 -m pytest -c pytest.ini`,
		`docker exec container go test ./...`,
		`docker run --rm image node --test`,
		`kubectl exec pod -- npm run build`,
		`find . -name '*.go' -exec gofmt -w {} +`,
		`xargs -0 -n 1 git status --short`,
		`go test ./... > /tmp/test.log 2>&1`,
		`desktop="$(xdg-user-dir DESKTOP)"; printf hello > "$desktop/file.txt"`,
		`printf '%s' "$(printf '%s' '$(node -e code)')"`,
	} {
		t.Run(command, func(t *testing.T) {
			if err := shellpolicy.Validate(command); err != nil {
				t.Fatalf("existing runner rejected: %v", err)
			}
		})
	}
}
