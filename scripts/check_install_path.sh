#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
gopath_marker='# solomon-installer-gopath'

path_has_dir() {
  local dir="$1" part
  dir="$(cd "$dir" 2>/dev/null && pwd -P || echo "$dir")"
  IFS=':' read -ra parts <<< "${PATH:-}"
  for part in "${parts[@]}"; do
    [[ -z "$part" ]] && continue
    if [[ "$(cd "$part" 2>/dev/null && pwd -P || echo "$part")" == "$dir" ]]; then
      return 0
    fi
  done
  return 1
}

strip_path_dir() {
  local dir="$1" cleaned
  PATH=":${PATH:-}:"
  PATH="${PATH//:${dir}:/:}"
  PATH="${PATH#:}"
  PATH="${PATH%:}"
  export PATH
}

verify_unix_rc_files() {
  local rc
  for rc in \
    "${HOME}/.zshrc" \
    "${HOME}/.profile"; do
    [[ -f "$rc" ]] || {
      echo "missing shell rc file: ${rc}" >&2
      exit 1
    }
    grep -Fq "$gopath_marker" "$rc" || {
      echo "missing gopath marker in ${rc}" >&2
      exit 1
    }
    grep -Fq 'go env GOBIN' "$rc" || {
      echo "missing GOBIN handling in ${rc}" >&2
      exit 1
    }
  done
  if [[ "$(uname -s)" == Darwin ]]; then
    rc="${HOME}/.bash_profile"
  else
    rc="${HOME}/.bashrc"
  fi
  [[ -f "$rc" ]] || {
    echo "missing shell rc file: ${rc}" >&2
    exit 1
  }
  grep -Fq "$gopath_marker" "$rc" || {
    echo "missing gopath marker in ${rc}" >&2
    exit 1
  }
  grep -Fq 'go env GOBIN' "$rc" || {
    echo "missing GOBIN handling in ${rc}" >&2
    exit 1
  }
}

verify_go_bin_in_path() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"
  path_has_dir "$bin_dir" || {
    echo "Go install bin not in PATH: ${bin_dir}" >&2
    echo "PATH=${PATH}" >&2
    exit 1
  }
}

verify_github_path_entry() {
  local bin_dir="$1"
  [[ -n "${GITHUB_PATH:-}" && -f "$GITHUB_PATH" ]] || {
    echo "GITHUB_PATH file missing" >&2
    exit 1
  }
  grep -Fxq "$bin_dir" "$GITHUB_PATH" || {
    echo "Go install bin not persisted to GITHUB_PATH: ${bin_dir}" >&2
    echo "GITHUB_PATH=$(cat "$GITHUB_PATH")" >&2
    exit 1
  }
}

run_path_setup_case() {
  local label="$1" local_go_bin
  shift
  echo "Checking install PATH setup (${label})..."
  "$@"
  export GITHUB_PATH="${HOME}/github-path"
  : >"$GITHUB_PATH"
  # shellcheck source=/dev/null
  source "${root}/scripts/install.sh"
  setup_path_only
  verify_go_bin_in_path "$expected_bin_dir"
  verify_github_path_entry "$expected_bin_dir"
  local_go_bin="${HOME}/.local/go/bin"
  mkdir -p "$local_go_bin"
  INSTALLED_LOCAL_GO=1
  ensure_local_go_in_path
  verify_github_path_entry "$local_go_bin"
  verify_unix_rc_files
}

base_home="${RUNNER_TEMP:-/tmp}/solomon-path-check-gopath"
rm -rf "$base_home"
mkdir -p "$base_home"
export HOME="$base_home"
export PATH="$(go env GOROOT)/bin:/usr/bin:/bin"
expected_bin_dir="$(go env GOPATH)/bin"
strip_path_dir "$expected_bin_dir"
unset GOBIN
run_path_setup_case "GOPATH/bin"

gobin_home="${RUNNER_TEMP:-/tmp}/solomon-path-check-gobin"
rm -rf "$gobin_home"
mkdir -p "$gobin_home"
export HOME="$gobin_home"
export PATH="$(go env GOROOT)/bin:/usr/bin:/bin"
export GOBIN="${gobin_home}/custom-go-bin"
expected_bin_dir="$GOBIN"
mkdir -p "$expected_bin_dir"
strip_path_dir "$(go env GOPATH)/bin"
strip_path_dir "$expected_bin_dir"
run_path_setup_case "GOBIN"

echo "install PATH setup OK"
