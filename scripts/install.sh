#!/usr/bin/env bash
set -euo pipefail

GO_REQUIRED="1.25.0"
GO_INSTALL_ROOT="${HOME}/.local/go"
INSTALLED_LOCAL_GO=0
INSTALL_VERSION="${SOLOMON_VERSION:-${1:-latest}}"
GITHUB_RELEASES_API="https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest"
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

resolve_latest_release_tag() {
  local tag api="$GITHUB_RELEASES_API"
  if command -v python3 >/dev/null 2>&1; then
    tag="$(curl -fsSL "$api" | python3 -c "import json,sys; print(json.load(sys.stdin)['tag_name'])")"
  else
    tag="$(curl -fsSL "$api" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  fi
  tag="${tag//$'\r'/}"
  tag="${tag//$'\n'/}"
  if [[ -z "$tag" ]]; then
    echo "failed to resolve latest GitHub release tag" >&2
    exit 1
  fi
  echo "$tag"
}

resolve_install_version() {
  if [[ "$INSTALL_VERSION" != latest ]]; then
    return 0
  fi
  INSTALL_VERSION="$(resolve_latest_release_tag)"
  echo "Latest release: ${INSTALL_VERSION}"
}

install_release_asset() {
  local platform goos goarch asset url bin_dir target tmp
  platform="$(detect_platform)"
  goos="${platform%-*}"
  goarch="${platform#*-}"
  asset="solomon-${INSTALL_VERSION}-${goos}-${goarch}"
  url="https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/${INSTALL_VERSION}/${asset}"
  bin_dir="$(go env GOPATH)/bin"
  target="${bin_dir}/solomon"
  tmp="$(mktemp)"
  mkdir -p "$bin_dir"
  echo "Downloading Solomon release asset ${asset}..."
  attempt=1
  max_attempts=15
  while true; do
    if curl -fsSL "$url" -o "$tmp"; then
      break
    fi
    if (( attempt >= max_attempts )); then
      echo "Failed to download ${asset} after ${max_attempts} attempts" >&2
      rm -f "$tmp"
      exit 1
    fi
    echo "Download failed (attempt ${attempt}/${max_attempts}), retrying..." >&2
    sleep 2
    attempt=$((attempt + 1))
  done
  mv "$tmp" "$target"
  chmod +x "$target"
}

install_go() {
  local platform tarball url parent tmp
  platform="$(detect_platform)"
  tarball="go${GO_REQUIRED}.${platform}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  parent="$(dirname "$GO_INSTALL_ROOT")"
  mkdir -p "$parent"
  echo "Downloading Go ${GO_REQUIRED} (${platform})..."
  tmp="$(mktemp -d)"
  curl -fsSL "$url" -o "${tmp}/${tarball}"
  rm -rf "$GO_INSTALL_ROOT"
  tar -C "$parent" -xzf "${tmp}/${tarball}"
  rm -rf "$tmp"
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

ensure_make() {
  if command -v make > /dev/null 2>&1; then
    echo "make OK ($(command -v make))"
    return 0
  fi
  echo "make not found; please install it and rerun this script." >&2
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin)
      echo "macOS: install Xcode Command Line Tools with 'xcode-select --install', or install make via Homebrew/MacPorts." >&2
      ;;
    linux)
      echo "Linux: install make with your package manager (e.g. 'sudo apt-get install -y make', 'sudo dnf install -y make', 'sudo pacman -S make')." >&2
      ;;
    *)
      echo "make is optional by itself; install it with your system package manager or manually if you want build tooling on $os." >&2
      ;;
  esac
  exit 1
}

node_lts_major() {
  local major=""
  if command -v curl >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
    major="$(curl -fsSL https://nodejs.org/dist/index.json 2>/dev/null | python3 -c "
import json,sys
for e in json.load(sys.stdin):
    if e.get('lts'):
        print(e['version'].lstrip('v').split('.')[0])
        break
" 2>/dev/null || true)"
  fi
  if [[ -z "$major" ]]; then
    major="22"
  fi
  echo "$major"
}

install_node_darwin() {
  local major formula brew_prefix
  major="$(node_lts_major)"
  if command -v brew >/dev/null 2>&1; then
    formula="node@${major}"
    echo "Installing Node.js LTS (${formula}) via Homebrew..."
    if ! brew install "$formula"; then
      echo "${formula} unavailable; trying brew install node..."
      brew install node
    else
      brew link --overwrite --force "$formula" 2>/dev/null || true
    fi
    if ! command -v node >/dev/null 2>&1; then
      brew_prefix="$(brew --prefix "$formula" 2>/dev/null || brew --prefix node 2>/dev/null || true)"
      if [[ -n "$brew_prefix" && -d "${brew_prefix}/bin" ]]; then
        export PATH="${brew_prefix}/bin:${PATH}"
      fi
    fi
    return 0
  fi
  if command -v port >/dev/null 2>&1; then
    echo "Installing Node.js LTS (nodejs${major}) via MacPorts..."
    if sudo port install "nodejs${major}"; then
      return 0
    fi
    for fallback in 22 20 18; do
      if [[ "$fallback" != "$major" ]] && sudo port install "nodejs${fallback}"; then
        return 0
      fi
    done
    echo "MacPorts Node.js install failed" >&2
    exit 1
  fi
  echo "Neither Homebrew nor MacPorts found; install Node.js LTS from https://nodejs.org/" >&2
  exit 1
}

install_node_linux() {
  local id="" id_like=""
  if [[ -f /etc/os-release ]]; then
    id="$(grep -E '^ID=' /etc/os-release | cut -d= -f2- | tr -d '"')"
    id_like="$(grep -E '^ID_LIKE=' /etc/os-release | cut -d= -f2- | tr -d '"' || true)"
  fi
  id="$(echo "$id" | tr '[:upper:]' '[:lower:]')"
  id_like="$(echo "$id_like" | tr '[:upper:]' '[:lower:]')"

  case "$id" in
    alpine)
      sudo apk add nodejs npm
      ;;
    arch | manjaro | endeavouros)
      sudo pacman -S --needed --noconfirm nodejs npm
      ;;
    fedora | rhel | centos | rocky | almalinux)
      sudo dnf install -y nodejs npm
      ;;
    opensuse-tumbleweed | opensuse-leap)
      sudo zypper install -y nodejs npm
      ;;
    ubuntu | debian | linuxmint | pop)
      sudo apt-get update && sudo apt-get install -y nodejs npm
      ;;
    *)
      if [[ "$id" == opensuse* ]]; then
        sudo zypper install -y nodejs npm
      elif [[ "$id_like" == *fedora* || "$id_like" == *rhel* ]]; then
        sudo dnf install -y nodejs npm
      elif [[ "$id_like" == *debian* || "$id_like" == *ubuntu* ]]; then
        sudo apt-get update && sudo apt-get install -y nodejs npm
      elif command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y nodejs npm
      elif command -v apt >/dev/null 2>&1; then
        sudo apt update && sudo apt install -y nodejs npm
      elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y nodejs npm
      elif command -v yum >/dev/null 2>&1; then
        sudo yum install -y nodejs npm
      elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --needed --noconfirm nodejs npm
      elif command -v zypper >/dev/null 2>&1; then
        sudo zypper install -y nodejs npm
      elif command -v apk >/dev/null 2>&1; then
        sudo apk add nodejs npm
      else
        echo "Unsupported Linux distro; install Node.js LTS from https://nodejs.org/" >&2
        exit 1
      fi
      ;;
  esac
}

ensure_node() {
  if command -v node >/dev/null 2>&1; then
    echo "Node $(node --version 2>/dev/null | tr -d '\r') OK"
    return 0
  fi
  echo "Node not found; installing LTS..."
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) install_node_darwin ;;
    linux) install_node_linux ;;
    *)
      echo "unsupported OS for Node install: $os (use scripts/install.ps1 on Windows)" >&2
      exit 1
      ;;
  esac
  if ! command -v node >/dev/null 2>&1; then
    echo "Node install failed; node not in PATH" >&2
    exit 1
  fi
  echo "Node $(node --version 2>/dev/null | tr -d '\r') ready"
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
  resolve_install_version
  echo "Installing solomon (${INSTALL_VERSION})..."
  install_release_asset
  if command -v solomon >/dev/null 2>&1; then
    echo "solomon installed: $(command -v solomon)"
    solomon version 2>/dev/null || true
  else
    echo "solomon binary is in $(go env GOPATH)/bin (add to PATH if needed)"
  fi
}

main() {
  ensure_go
  ensure_make
  setup_shell
  install_solomon
  echo "Done."
}

main "$@"
