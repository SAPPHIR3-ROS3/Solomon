#!/usr/bin/env bash
set -euo pipefail

: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

repo="${GITHUB_REPOSITORY}"

fetch_prev_release() {
  local attempt prev=""
  for attempt in 1 2 3 4 5; do
    if [[ -n "${GH_TOKEN:-}" ]] && command -v gh >/dev/null 2>&1; then
      prev="$(gh api "/repos/${repo}/releases?per_page=2" --jq '.[1].tag_name // empty' 2>/dev/null || true)"
    else
      local auth_header=()
      if [[ -n "${GH_TOKEN:-}" ]]; then
        auth_header=(-H "Authorization: Bearer ${GH_TOKEN}")
      fi
      prev="$(curl -fsSL -H "User-Agent: solomon-upgrade-smoke" "${auth_header[@]}" \
        "https://api.github.com/repos/${repo}/releases?per_page=2" | jq -r '.[1].tag_name // empty' 2>/dev/null || true)"
    fi
    if [[ -n "$prev" ]]; then
      printf '%s' "$prev"
      return 0
    fi
    echo "release lookup failed (attempt ${attempt}/5); retrying..." >&2
    sleep 2
  done
  return 1
}

prev="$(fetch_prev_release || true)"
if [[ -z "$prev" || "$prev" == "$RELEASE_TAG" ]]; then
  echo "Skipping upgrade smoke: no previous release to upgrade from (prev=${prev:-none})"
  exit 0
fi

echo "Upgrade smoke: ${prev} -> ${RELEASE_TAG}"
curl -fsSL "https://raw.githubusercontent.com/${repo}/main/scripts/install.sh" | bash -s -- "$prev"

bin_dir="$(go env GOPATH)/bin"
gobin="$(go env GOBIN 2>/dev/null | tr -d '\r\n')"
if [[ -n "$gobin" ]]; then
  bin_dir="$gobin"
fi
exe="${bin_dir}/solomon"
export NO_COLOR=1

current="$("$exe" version | tr -d '\r\n')"
echo "Installed previous release: ${current}"
if [[ "$current" != *"$prev"* ]]; then
  echo "expected version to include ${prev}, got ${current}" >&2
  exit 1
fi
if ! strings "$exe" | grep -q 'solomon-cli-upgrade-v1'; then
  echo "Skipping upgrade smoke: ${prev} predates solomon upgrade CLI"
  exit 0
fi

"$exe" upgrade &
upgrade_pid=$!
for attempt in $(seq 1 90); do
  if ver="$("$exe" version 2>/dev/null | tr -d '\r\n')" && [[ "$ver" == *"$RELEASE_TAG"* ]]; then
    echo "Upgrade smoke OK: ${ver}"
    wait "$upgrade_pid" 2>/dev/null || true
    exit 0
  fi
  sleep 2
done

echo "Upgrade smoke failed: expected version to include ${RELEASE_TAG}" >&2
"$exe" version >&2 || true
wait "$upgrade_pid" 2>/dev/null || true
exit 1
