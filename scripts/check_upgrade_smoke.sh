#!/usr/bin/env bash
set -euo pipefail

: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

repo="${GITHUB_REPOSITORY}"
legacy_tag="${UPGRADE_SMOKE_LEGACY_TAG:-v2026.624.0}"
fixed_baseline="${UPGRADE_SMOKE_FIXED_BASELINE:-v2026.701.0}"
smoke_root="${UPGRADE_SMOKE_ROOT:-$(mktemp -d)}"

strict_error_patterns=(
  'InvalidOperation'
  'RestartExe'
  'Cannot create a file when that file already exists'
)

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

release_exists() {
  local tag="$1"
  curl -fsI "https://github.com/${repo}/releases/tag/${tag}" >/dev/null 2>&1
}

version_ge() {
  local left="${1#v}"
  local right="${2#v}"
  local first
  first="$(printf '%s\n%s\n' "$left" "$right" | sort -V | tail -n1)"
  [[ "$first" == "$left" ]]
}

has_cli_marker() {
  local exe="$1"
  strings "$exe" | grep -q 'solomon-cli-upgrade-v1'
}

verify_log_strict() {
  local log="$1"
  local from_tag="$2"
  if ! version_ge "$from_tag" "$fixed_baseline"; then
    return 0
  fi
  local pattern
  for pattern in "${strict_error_patterns[@]}"; do
    if grep -q "$pattern" "$log"; then
      echo "upgrade smoke: forbidden output for ${from_tag}: ${pattern}" >&2
      cat "$log" >&2
      return 1
    fi
  done
}

case_dir_for() {
  local from_tag="$1"
  local method="$2"
  printf '%s/%s-%s' "$smoke_root" "${from_tag//\//_}" "$method"
}

default_bin_dir() {
  local bin_dir gobin
  bin_dir="$(go env GOPATH)/bin"
  gobin="$(go env GOBIN 2>/dev/null | tr -d '\r\n')"
  if [[ -n "$gobin" ]]; then
    bin_dir="$gobin"
  fi
  printf '%s' "$bin_dir"
}

install_release() {
  local from_tag="$1"
  unset GOBIN
  curl -fsSL "https://raw.githubusercontent.com/${repo}/main/scripts/install.sh" | bash -s -- "$from_tag"
}

exe_path() {
  printf '%s/solomon' "$(default_bin_dir)"
}

wait_for_target_version() {
  local exe="$1"
  local attempt ver=""
  for attempt in $(seq 1 90); do
    if ver="$("$exe" version 2>/dev/null | tr -d '\r\n')" && [[ "$ver" == *"$RELEASE_TAG"* ]]; then
      echo "Upgrade smoke OK (${RELEASE_TAG}): ${ver}"
      return 0
    fi
    sleep 2
  done
  echo "Upgrade smoke failed: expected version to include ${RELEASE_TAG}, last=${ver:-none}" >&2
  "$exe" version >&2 || true
  return 1
}

run_cli_upgrade() {
  local exe="$1"
  local log="$2"
  "$exe" upgrade >"$log" 2>&1 &
  local upgrade_pid=$!
  wait_for_target_version "$exe"
  wait "$upgrade_pid" 2>/dev/null || true
}

run_repl_upgrade() {
  local exe="$1"
  local log="$2"
  python3 "$(dirname "$0")/repl_upgrade_pty.py" "$exe" >"$log" 2>&1 &
  local upgrade_pid=$!
  wait_for_target_version "$exe"
  wait "$upgrade_pid" 2>/dev/null || true
}

run_case() {
  local from_tag="$1"
  local method="$2"
  local log
  log="$(case_dir_for "$from_tag" "$method").log"

  echo "Upgrade smoke (${method}): ${from_tag} -> ${RELEASE_TAG}"
  install_release "$from_tag"
  local exe
  exe="$(exe_path)"
  export NO_COLOR=1

  local current
  current="$("$exe" version | tr -d '\r\n')"
  echo "Installed source release: ${current}"
  if [[ "$current" != *"$from_tag"* ]]; then
    echo "expected version to include ${from_tag}, got ${current}" >&2
    return 1
  fi

  if [[ "$method" == "cli" ]]; then
    if ! has_cli_marker "$exe"; then
      echo "Skipping CLI upgrade smoke for ${from_tag}: no solomon upgrade CLI"
      return 0
    fi
    run_cli_upgrade "$exe" "$log"
  else
    run_repl_upgrade "$exe" "$log"
  fi

  verify_log_strict "$log" "$from_tag"
}

prev="$(fetch_prev_release || true)"
if [[ -z "$prev" ]] && ! release_exists "$legacy_tag"; then
  echo "Skipping upgrade smoke: no previous or legacy release to upgrade from"
  exit 0
fi

sources=()
if [[ -n "$prev" && "$prev" != "$RELEASE_TAG" ]]; then
  sources+=("$prev")
fi
if [[ "$legacy_tag" != "$RELEASE_TAG" && "$legacy_tag" != "$prev" ]] && release_exists "$legacy_tag"; then
  sources+=("$legacy_tag")
fi
if [[ ${#sources[@]} -eq 0 ]]; then
  echo "Skipping upgrade smoke: no source tags to test"
  exit 0
fi

mkdir -p "$smoke_root"

for from_tag in "${sources[@]}"; do
  if [[ "$from_tag" == "$RELEASE_TAG" ]]; then
    continue
  fi
  run_case "$from_tag" "cli"
  run_case "$from_tag" "repl"
done
