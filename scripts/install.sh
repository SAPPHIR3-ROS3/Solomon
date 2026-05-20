#!/usr/bin/env bash
set -euo pipefail

GO_REQUIRED="1.25.0"
GO_INSTALL_ROOT="${HOME}/.local/go"
INSTALLED_LOCAL_GO=0
SOLMON_PKG="github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest"
MARKER="# solomon-installer"

version_ge() {
  local have="$1" want="$2"
  if [[ "$(printf '%s\n%s\n' "$want" "$have" | sort -V | head -n1)" == "$want" ]]; then
    return 0
  fi
  return 1
}

go_semver() {
  local raw
  raw="$(go version 2>/dev/null | awk '{print $3}')"
  raw="${raw#go}"
  raw="${raw%%-*}"
  echo "$raw"
}

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
    x86_64 | amd64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
  case "$os" in
    linux) echo "linux-${arch}" ;;
    darwin) echo "darwin-${arch}" ;;
    *)
      echo "unsupported OS: $os (use scripts/install.ps1 on Windows)" >&2
      exit 1
      ;;
  esac
}

install_go() {
  local platform tarball url parent
  platform="$(detect_platform)"
  tarball="go${GO_REQUIRED}.${platform}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  parent="$(dirname "$GO_INSTALL_ROOT")"
  mkdir -p "$parent"
  echo "Downloading Go ${GO_REQUIRED} (${platform})..."
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  curl -fsSL "$url" -o "${tmp}/${tarball}"
  rm -rf "$GO_INSTALL_ROOT"
  tar -C "$parent" -xzf "${tmp}/${tarball}"
  export PATH="${GO_INSTALL_ROOT}/bin:${PATH}"
  INSTALLED_LOCAL_GO=1
}

ensure_go() {
  local ver
  if command -v go >/dev/null 2>&1; then
    ver="$(go_semver)"
    if version_ge "$ver" "$GO_REQUIRED"; then
      echo "Go ${ver} OK (>= ${GO_REQUIRED})"
      return 0
    fi
    echo "Go ${ver} is older than ${GO_REQUIRED}; upgrading..."
  else
    echo "Go not found; installing ${GO_REQUIRED}..."
  fi
  install_go
  ver="$(go_semver)"
  if ! version_ge "$ver" "$GO_REQUIRED"; then
    echo "Go install failed (got ${ver})" >&2
    exit 1
  fi
  echo "Go ${ver} ready"
}

rc_file() {
  local shell_name="${SHELL:-}"
  shell_name="${shell_name##*/}"
  case "$shell_name" in
    zsh) echo "${HOME}/.zshrc" ;;
    bash)
      if [[ "$(uname -s)" == Darwin ]]; then
        echo "${HOME}/.bash_profile"
      else
        echo "${HOME}/.bashrc"
      fi
      ;;
    fish) echo "${HOME}/.config/fish/config.fish" ;;
    *) echo "${HOME}/.profile" ;;
  esac
}

line_in_file() {
  local file="$1" line="$2"
  [[ -f "$file" ]] && grep -Fq "$line" "$file"
}

append_unix_path() {
  local rc go_bin gopath_bin
  rc="$(rc_file)"
  mkdir -p "$(dirname "$rc")"
  touch "$rc"

  if [[ "$INSTALLED_LOCAL_GO" == 1 ]]; then
    go_bin='export PATH="$HOME/.local/go/bin:$PATH"'
    if ! line_in_file "$rc" "$go_bin" && ! line_in_file "$rc" "$MARKER-go"; then
      {
        echo ""
        echo "$MARKER-go"
        echo "$go_bin"
      } >>"$rc"
      echo "Added Go binary path to ${rc}"
    fi
  fi

  gopath_bin='export PATH="$PATH:$(go env GOPATH)/bin"'
  if ! line_in_file "$rc" '$(go env GOPATH)/bin' && ! line_in_file "$rc" "$MARKER-gopath"; then
    {
      echo ""
      echo "$MARKER-gopath"
      echo "$gopath_bin"
    } >>"$rc"
    echo "Added GOPATH/bin to ${rc}"
  fi

  export PATH="${PATH}:$(go env GOPATH)/bin"
}

append_fish_path() {
  local rc
  rc="$(rc_file)"
  mkdir -p "$(dirname "$rc")"
  touch "$rc"

  if [[ "$INSTALLED_LOCAL_GO" == 1 ]]; then
    if ! line_in_file "$rc" "$MARKER-go"; then
      {
        echo ""
        echo "$MARKER-go"
        echo 'fish_add_path $HOME/.local/go/bin'
      } >>"$rc"
      echo "Added Go binary path to ${rc}"
    fi
  fi

  if ! line_in_file "$rc" "$MARKER-gopath"; then
    {
      echo ""
      echo "$MARKER-gopath"
      echo 'fish_add_path (go env GOPATH)/bin'
    } >>"$rc"
    echo "Added GOPATH/bin to ${rc}"
  fi

  export PATH="${PATH}:$(go env GOPATH)/bin"
}

setup_shell() {
  local shell_name="${SHELL:-}"
  shell_name="${shell_name##*/}"
  if [[ "$shell_name" == fish ]]; then
    append_fish_path
  else
    append_unix_path
  fi
  echo "Reload your shell or run: source $(rc_file)"
}

install_solomon() {
  echo "Installing solomon..."
  go install "$SOLMON_PKG"
  if command -v solomon >/dev/null 2>&1; then
    echo "solomon installed: $(command -v solomon)"
    solomon version 2>/dev/null || true
  else
    echo "solomon binary is in $(go env GOPATH)/bin (add to PATH if needed)"
  fi
}

main() {
  ensure_go
  setup_shell
  install_solomon
  echo "Done."
}

main "$@"
