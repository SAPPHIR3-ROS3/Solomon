#!/usr/bin/env bash
set -euo pipefail

GO_REQUIRED="1.25.0"
NODE_REQUIRED="20.0.0"
GO_INSTALL_ROOT="${HOME}/.local/go"
INSTALLED_LOCAL_GO=0
INSTALL_VERSION="${SOLOMON_VERSION:-${1:-latest}}"
GITHUB_RELEASES_API="https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest"
MARKER="# solomon-installer"
DEFAULT_SERVER_PORT="64000"

go_install_bin_dir() {
  local gobin
  gobin="$(go env GOBIN 2>/dev/null || true)"
  gobin="${gobin//$'\r'/}"
  gobin="${gobin//$'\n'/}"
  if [[ -n "$gobin" ]]; then
    printf '%s\n' "$gobin"
    return
  fi
  printf '%s/bin\n' "$(go env GOPATH)"
}

persist_github_path() {
  local dir="$1"
  if [[ -z "${GITHUB_PATH:-}" || -z "$dir" ]]; then
    return 0
  fi
  printf '%s\n' "$dir" >>"$GITHUB_PATH"
}

ensure_local_go_in_path() {
  if [[ "$INSTALLED_LOCAL_GO" != 1 ]]; then
    return 0
  fi
  local go_bin="${HOME}/.local/go/bin"
  case ":${PATH}:" in
    *":${go_bin}:"*) ;;
    *) export PATH="${go_bin}:${PATH}" ;;
  esac
  persist_github_path "$go_bin"
}

ensure_go_bin_in_path() {
  local bin_dir
  ensure_local_go_in_path
  bin_dir="$(go_install_bin_dir)"
  mkdir -p "$bin_dir"
  case ":${PATH}:" in
    *":${bin_dir}:"*) ;;
    *) export PATH="${bin_dir}:${PATH}" ;;
  esac
  persist_github_path "$bin_dir"
}

go_install_bin_export_line() {
  cat <<'EOF'
if command -v go >/dev/null 2>&1; then
  _solomon_gobin="$(go env GOBIN 2>/dev/null | tr -d '\r\n')"
  if [[ -n "$_solomon_gobin" ]]; then
    _solomon_go_bin="$_solomon_gobin"
  else
    _solomon_go_bin="$(go env GOPATH)/bin"
  fi
  case ":${PATH}:" in
    *":${_solomon_go_bin}:"*) ;;
    *) export PATH="${_solomon_go_bin}:${PATH}" ;;
  esac
  unset _solomon_gobin _solomon_go_bin
fi
EOF
}

go_install_bin_fish_line() {
  cat <<'EOF'
if command -q go
  set -l _solomon_gobin (go env GOBIN 2>/dev/null | string trim)
  if test -n "$_solomon_gobin"
    fish_add_path --prepend $_solomon_gobin
  else
    fish_add_path --prepend (go env GOPATH)/bin
  end
end
EOF
}

GOPATH_MARKER="${MARKER}-gopath"

remove_rc_block_after_marker() {
  local rc="$1" marker="$2"
  [[ -f "$rc" ]] || return 0
  local tmp
  tmp="$(mktemp)"
  awk -v marker="$marker" '
    $0 == marker { skip=1; next }
    skip && /^[[:space:]]*$/ { skip=0; next }
    skip { next }
    { print }
  ' "$rc" > "$tmp" && mv "$tmp" "$rc"
}

ensure_gopath_rc_block() {
  local rc="$1" line body_fn
  body_fn="$2"
  line="$("$body_fn")"
  if line_in_file "$rc" "$GOPATH_MARKER" && line_in_file "$rc" 'go env GOBIN'; then
    return 0
  fi
  if line_in_file "$rc" "$GOPATH_MARKER"; then
    remove_rc_block_after_marker "$rc" "$GOPATH_MARKER"
    echo "Updated Go install bin directory in ${rc}"
  else
    echo "Added Go install bin directory to ${rc}"
  fi
  {
    echo ""
    echo "$GOPATH_MARKER"
    printf '%s\n' "$line"
  } >>"$rc"
}

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
  bin_dir="$(go_install_bin_dir)"
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
  checksums_url="https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/${INSTALL_VERSION}/checksums.txt"
  checksums="$(mktemp)"
  if curl -fsSL "$checksums_url" -o "$checksums"; then
    expected="$(awk -v asset="$asset" '$NF==asset {print $1; exit}' "$checksums")"
    if [[ -z "$expected" ]]; then
      echo "checksums: no entry for ${asset} in checksums.txt" >&2
      rm -f "$tmp" "$checksums"
      exit 1
    fi
    actual="$(sha256sum "$tmp" | awk '{print $1}')"
    if [[ "$expected" != "$actual" ]]; then
      echo "checksum mismatch for ${asset} (expected ${expected}, got ${actual})" >&2
      rm -f "$tmp" "$checksums"
      exit 1
    fi
    rm -f "$checksums"
  else
    echo "Warning: no checksums.txt for ${INSTALL_VERSION}; skipping integrity check" >&2
  fi
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
      ensure_go_bin_in_path
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
  ensure_go_bin_in_path
}

linux_install_packages() {
  local id="" id_like="" pkgs=("$@")
  if [[ -f /etc/os-release ]]; then
    id="$(grep -E '^ID=' /etc/os-release | cut -d= -f2- | tr -d '"')"
    id_like="$(grep -E '^ID_LIKE=' /etc/os-release | cut -d= -f2- | tr -d '"' || true)"
  fi
  id="$(echo "$id" | tr '[:upper:]' '[:lower:]')"
  id_like="$(echo "$id_like" | tr '[:upper:]' '[:lower:]')"

  case "$id" in
    alpine)
      sudo apk add "${pkgs[@]}"
      ;;
    arch | manjaro | endeavouros)
      sudo pacman -S --needed --noconfirm "${pkgs[@]}"
      ;;
    fedora | rhel | centos | rocky | almalinux)
      sudo dnf install -y "${pkgs[@]}"
      ;;
    opensuse-tumbleweed | opensuse-leap)
      sudo zypper install -y "${pkgs[@]}"
      ;;
    ubuntu | debian | linuxmint | pop)
      sudo apt-get update && sudo apt-get install -y "${pkgs[@]}"
      ;;
    *)
      if [[ "$id" == opensuse* ]]; then
        sudo zypper install -y "${pkgs[@]}"
      elif [[ "$id_like" == *fedora* || "$id_like" == *rhel* ]]; then
        sudo dnf install -y "${pkgs[@]}"
      elif [[ "$id_like" == *debian* || "$id_like" == *ubuntu* ]]; then
        sudo apt-get update && sudo apt-get install -y "${pkgs[@]}"
      elif command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y "${pkgs[@]}"
      elif command -v apt >/dev/null 2>&1; then
        sudo apt update && sudo apt install -y "${pkgs[@]}"
      elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y "${pkgs[@]}"
      elif command -v yum >/dev/null 2>&1; then
        sudo yum install -y "${pkgs[@]}"
      elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --needed --noconfirm "${pkgs[@]}"
      elif command -v zypper >/dev/null 2>&1; then
        sudo zypper install -y "${pkgs[@]}"
      elif command -v apk >/dev/null 2>&1; then
        sudo apk add "${pkgs[@]}"
      else
        return 1
      fi
      ;;
  esac
}

install_make_darwin() {
  local brew_prefix
  if command -v brew >/dev/null 2>&1; then
    echo "Installing make via Homebrew..."
    if brew install make; then
      brew_prefix="$(brew --prefix make 2>/dev/null || true)"
      if [[ -n "$brew_prefix" && -d "${brew_prefix}/libexec/gnubin" ]]; then
        export PATH="${brew_prefix}/libexec/gnubin:${PATH}"
      fi
      command -v make >/dev/null 2>&1 && return 0
    fi
  fi
  if command -v port >/dev/null 2>&1; then
    echo "Installing make via MacPorts..."
    if sudo port install make; then
      return 0
    fi
  fi
  if ! xcode-select -p >/dev/null 2>&1; then
    echo "Installing Xcode Command Line Tools (complete the dialog if shown, then rerun this script)..."
    xcode-select --install 2>/dev/null || true
    exit 1
  fi
  echo "make install failed; install via Homebrew, MacPorts, or Xcode Command Line Tools." >&2
  exit 1
}

install_make_linux() {
  if ! linux_install_packages make; then
    echo "Unsupported Linux distro; install make with your package manager." >&2
    exit 1
  fi
}

ensure_make() {
  if command -v make > /dev/null 2>&1; then
    echo "make OK ($(command -v make))"
    return 0
  fi
  echo "make not found; installing..."
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) install_make_darwin ;;
    linux) install_make_linux ;;
    *)
      echo "unsupported OS for make install: $os (use scripts/install.ps1 on Windows)" >&2
      exit 1
      ;;
  esac
  if ! command -v make >/dev/null 2>&1; then
    echo "make install failed; make not in PATH" >&2
    exit 1
  fi
  echo "make ready ($(command -v make))"
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

install_node_linux_fallback() {
  if ! linux_install_packages nodejs npm; then
    echo "Unsupported Linux distro; install Node.js LTS from https://nodejs.org/" >&2
    exit 1
  fi
}

install_node_linux() {
  local major
  major="$(node_lts_major)"

  if command -v curl >/dev/null 2>&1 && command -v apt-get >/dev/null 2>&1; then
    echo "Installing Node.js LTS (${major}.x) via NodeSource..."
    if curl -fsSL "https://deb.nodesource.com/setup_${major}.x" | sudo bash -; then
      if sudo apt-get install -y nodejs && command -v node >/dev/null 2>&1; then
        return 0
      fi
    fi
    echo "NodeSource install failed; trying distro package manager..." >&2
  fi

  install_node_linux_fallback
}

ensure_node() {
  local node_present=0
  if command -v node >/dev/null 2>&1; then
    node_present=1
    local ver
    ver="$(node --version 2>/dev/null | tr -d '\r' | sed 's/^v//' | sed 's/-.*$//')"
    if version_ge "$ver" "$NODE_REQUIRED"; then
      echo "Node ${ver} OK (>= ${NODE_REQUIRED})"
      return 0
    fi
    echo "Node ${ver} is older than ${NODE_REQUIRED}; upgrading..."
  fi
  if [[ "$node_present" == 1 ]]; then
    echo "Installing Node.js LTS..."
  else
    echo "Node not found; installing LTS..."
  fi
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
  local ver
  ver="$(node --version 2>/dev/null | tr -d '\r' | sed 's/^v//' | sed 's/-.*$//')"
  if ! version_ge "$ver" "$NODE_REQUIRED"; then
    echo "Node install failed (got ${ver}; need >= ${NODE_REQUIRED})" >&2
    exit 1
  fi
  echo "Node ${ver} ready"
}

install_cloakbrowser() {
  ensure_node
  if ! command -v npm >/dev/null 2>&1 || ! command -v npx >/dev/null 2>&1; then
    echo "npm and npx are required to install the official CloakBrowser wrapper" >&2
    exit 1
  fi
  local solomon_home cloak_dir cloak_cache
  solomon_home="${SOLOMON_HOME:-${HOME}/.solomon}"
  cloak_dir="${solomon_home}/cloakbrowser"
  cloak_cache="${cloak_dir}/cache"
  mkdir -p "$cloak_dir" "$cloak_cache"
  echo "Installing the official CloakBrowser wrapper..."
  (
    cd "$cloak_dir"
    npm install --ignore-scripts --no-audit --no-fund cloakbrowser playwright-core
    CLOAKBROWSER_CACHE_DIR="$cloak_cache" npx --no-install cloakbrowser install
  )
  echo "CloakBrowser ready: ${cloak_dir}"
}

load_installer_server_port_from_dotenv() {
  if [[ -n "${SOLOMON_SERVER_PORT:-}" || ! -f ".env" ]]; then
    return 0
  fi
  local value
  value="$(sed -nE 's/^[[:space:]]*(export[[:space:]]+)?SOLOMON_SERVER_PORT[[:space:]]*=[[:space:]]*([^#[:space:]]+).*$/\2/p' .env | head -n1)"
  value="${value#\"}"
  value="${value%\"}"
  value="${value#\'}"
  value="${value%\'}"
  if [[ -n "$value" ]]; then
    export SOLOMON_SERVER_PORT="$value"
  fi
}

server_port_from_config() {
  local config_path="$1"
  [[ -f "$config_path" ]] || return 0
  awk '
    /^[[:space:]]*\[/ { exit }
    /^[[:space:]]*server_port[[:space:]]*=/ {
      value = $0
      sub(/^[^=]*=/, "", value)
      sub(/[[:space:]]*#.*/, "", value)
      gsub(/[[:space:]]/, "", value)
      print value
      exit
    }
  ' "$config_path"
}

has_top_level_toml_scalar() {
  local file="$1" key="$2"
  [[ -f "$file" ]] || return 1
  awk -v key="$key" '
    /^[[:space:]]*\[/ { exit }
    $0 ~ "^[[:space:]]*" key "[[:space:]]*=" { found = 1; exit }
    END { exit(found ? 0 : 1) }
  ' "$file"
}

resolve_installer_server_port() {
  local solomon_home="$1" config_path raw
  load_installer_server_port_from_dotenv
  config_path="${solomon_home}/config.toml"
  raw="${SOLOMON_SERVER_PORT:-}"
  if [[ -z "$raw" ]]; then
    raw="$(server_port_from_config "$config_path")"
  fi
  if [[ -z "$raw" ]]; then
    raw="$DEFAULT_SERVER_PORT"
  fi
  if ! [[ "$raw" =~ ^[0-9]+$ ]] || (( raw < 1 || raw > 65535 )); then
    echo "SOLOMON_SERVER_PORT must be a TCP port between 1 and 65535" >&2
    exit 1
  fi
  printf '%s\n' "$raw"
}

upsert_toml_scalar() {
  local file="$1" key="$2" value="$3" line tmp
  line="${key} = ${value}"
  mkdir -p "$(dirname "$file")"
  tmp="$(mktemp)"
  if [[ -f "$file" ]]; then
    awk -v key="$key" -v line="$line" '
      BEGIN { in_table = 0; updated = 0 }
      /^[[:space:]]*\[/ {
        if (!updated) { print line; updated = 1 }
        in_table = 1
        print
        next
      }
      $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
        if (!in_table && !updated) { print line; updated = 1 }
        next
      }
      { print }
      END { if (!updated) print line }
    ' "$file" >"$tmp"
  else
    printf '%s\n' "$line" >"$tmp"
  fi
  chmod 600 "$tmp"
  mv "$tmp" "$file"
}

ensure_mcp_web_config() {
  local solomon_home="$1" mcp_path="${1}/mcp.json"
  mkdir -p "$solomon_home"
  if [[ ! -f "$mcp_path" ]]; then
    (
      umask 077
      cat >"$mcp_path" <<'EOF'
{
  "mcpServers": {
    "exa-web": {
      "type": "streamable-http",
      "url": "https://mcp.exa.ai/mcp",
      "internal": true,
      "adapter": "exa",
      "allow": ["web_search_exa", "web_fetch_exa"]
    },
    "parallel-web": {
      "type": "streamable-http",
      "url": "https://search.parallel.ai/mcp",
      "internal": true,
      "adapter": "parallel",
      "allow": ["web_search", "web_fetch"]
    }
  }
}
EOF
    )
    chmod 600 "$mcp_path"
    return 0
  fi

  command -v python3 >/dev/null 2>&1 || {
    echo "python3 is required to update the existing MCP configuration" >&2
    exit 1
  }
  python3 - "$mcp_path" <<'PY'
import json
import os
import sys
import tempfile

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as handle:
    document = json.load(handle)
servers = document.get("mcpServers")
if servers is None:
    servers = {}
if not isinstance(servers, dict):
    raise SystemExit("mcp.json: mcpServers must be an object")
servers.update({
    "exa-web": {
        "type": "streamable-http",
        "url": "https://mcp.exa.ai/mcp",
        "internal": True,
        "adapter": "exa",
        "allow": ["web_search_exa", "web_fetch_exa"],
    },
    "parallel-web": {
        "type": "streamable-http",
        "url": "https://search.parallel.ai/mcp",
        "internal": True,
        "adapter": "parallel",
        "allow": ["web_search", "web_fetch"],
    },
})
document["mcpServers"] = servers
directory = os.path.dirname(path) or "."
fd, temporary = tempfile.mkstemp(prefix=".mcp-", dir=directory, text=True)
try:
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(document, handle, indent=2)
        handle.write("\n")
    os.chmod(temporary, 0o600)
    os.replace(temporary, path)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY
  chmod 600 "$mcp_path"
}

configure_runtime_defaults() {
  local solomon_home config_path port
  solomon_home="${SOLOMON_HOME:-${HOME}/.solomon}"
  config_path="${solomon_home}/config.toml"
  mkdir -p "$solomon_home"
  if [[ ! -f "$config_path" ]]; then
    : >"$config_path"
    chmod 600 "$config_path"
  fi
  port="$(resolve_installer_server_port "$solomon_home")"
  upsert_toml_scalar "$config_path" "server_port" "$port"
  if ! has_top_level_toml_scalar "$config_path" "web_search_engine"; then
    upsert_toml_scalar "$config_path" "web_search_engine" '"internal"'
  fi
  ensure_mcp_web_config "$solomon_home"
  echo "Solomon web backend: internal (Exa/Parallel + CloakBrowser fallback)"
  echo "Solomon server port: ${port}"
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

unix_rc_files() {
  echo "${HOME}/.zshrc"
  if [[ "$(uname -s)" == Darwin ]]; then
    echo "${HOME}/.bash_profile"
  else
    echo "${HOME}/.bashrc"
  fi
  echo "${HOME}/.profile"
}

configure_unix_rc() {
  local rc go_bin
  rc="$1"
  [[ -n "$rc" ]] || return 0
  mkdir -p "$(dirname "$rc")"
  touch "$rc"

  if [[ "$INSTALLED_LOCAL_GO" == 1 ]]; then
    go_bin='export PATH="$HOME/.local/go/bin:$PATH"'
    if ! line_in_file "$rc" "$MARKER-go"; then
      {
        echo ""
        echo "$MARKER-go"
        echo "$go_bin"
      } >>"$rc"
      echo "Added Go binary path to ${rc}"
    fi
  fi

  ensure_gopath_rc_block "$rc" go_install_bin_export_line
}

configure_fish_rc() {
  local rc
  rc="$1"
  [[ -n "$rc" ]] || return 0
  mkdir -p "$(dirname "$rc")"
  touch "$rc"

  if [[ "$INSTALLED_LOCAL_GO" == 1 ]]; then
    if ! line_in_file "$rc" "$MARKER-go"; then
      {
        echo ""
        echo "$MARKER-go"
        echo 'fish_add_path --prepend $HOME/.local/go/bin'
      } >>"$rc"
      echo "Added Go binary path to ${rc}"
    fi
  fi

  ensure_gopath_rc_block "$rc" go_install_bin_fish_line
}

setup_shell() {
  local shell_name="${SHELL:-}" rc
  shell_name="${shell_name##*/}"
  if [[ "$shell_name" == fish ]]; then
    configure_fish_rc "${HOME}/.config/fish/config.fish"
  fi
  while IFS= read -r rc; do
    configure_unix_rc "$rc"
  done < <(unix_rc_files)
  ensure_go_bin_in_path
  echo "Reload your shell or run: source $(rc_file)"
}

install_solomon() {
  resolve_install_version
  ensure_go_bin_in_path
  echo "Installing solomon (${INSTALL_VERSION})..."
  install_release_asset
  local bin_dir solomon_bin
  bin_dir="$(go_install_bin_dir)"
  solomon_bin="${bin_dir}/solomon"
  if command -v solomon >/dev/null 2>&1; then
    echo "solomon installed: $(command -v solomon)"
    solomon init 2>/dev/null || true
    solomon version 2>/dev/null || true
  elif [[ -x "$solomon_bin" ]]; then
    echo "solomon installed: ${solomon_bin}"
    "$solomon_bin" init 2>/dev/null || true
    "$solomon_bin" version 2>/dev/null || true
  else
    echo "solomon binary is in ${bin_dir} (add to PATH if needed)" >&2
    exit 1
  fi
}

setup_path_only() {
  ensure_go
  setup_shell
}

main() {
  ensure_go
  ensure_make
  setup_shell
  install_solomon
  install_cloakbrowser
  configure_runtime_defaults
  echo "Done."
}

if [[ "${1:-}" == "--setup-path-only" ]]; then
  setup_path_only
  exit 0
fi

if [[ "${BASH_SOURCE[0]:-${0}}" == "${0}" ]]; then
  main "$@"
fi
