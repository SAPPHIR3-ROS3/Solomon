#!/usr/bin/env bash
set -euo pipefail

GO_REQUIRED="1.25.0"
NODE_REQUIRED="20.0.0"
GO_INSTALL_ROOT="${HOME}/.local/go"
INSTALLED_LOCAL_GO=0
BREW_BIN_DIR=""
NODE_BIN_DIR=""
MAKE_BIN_DIR=""
INSTALL_VERSION="${SOLOMON_VERSION:-${1:-latest}}"
GITHUB_RELEASES_API="https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest"
MARKER="# solomon-installer"
DEFAULT_SERVER_PORT="64000"

installer_step() {
  INSTALL_STEP="$1"
  INSTALL_OPERATION="$1"
  INSTALL_HINT="$2"
  printf '[install] %s\n' "$INSTALL_STEP" >&2
}

installer_error() {
  local status="$1" line="${2:-unknown}" command="${3:-explicit exit}" caller="${4:-main}"
  INSTALL_ERROR_REPORTED=1
  printf '\n[install] FAILED: %s\nOperation: %s\nExit code: %s\nLocation: %s, line %s\nCommand: %s\nNext: %s\nOriginal command output is shown above. Resolve this error and rerun the same install command.\n' \
    "${INSTALL_STEP:-Installer startup}" "${INSTALL_OPERATION:-Initialize installer}" "$status" "$caller" "$line" "$command" \
    "${INSTALL_HINT:-Check the error above, required commands, filesystem permissions and available disk space.}" >&2
}

installer_enable_diagnostics() {
  set -E
  INSTALL_ERROR_REPORTED=0
  # ERR identifies unexpected failures; EXIT also covers explicit exit guards.
  # Existing cleanup and rollback traps in subshells continue to run unchanged.
  trap 'installer_error "$?" "$LINENO" "$BASH_COMMAND" "${FUNCNAME[0]:-main}"' ERR
  trap 'status=$?; if (( status != 0 )) && [[ "${INSTALL_ERROR_REPORTED:-0}" != 1 ]]; then installer_error "$status"; fi' EXIT
}

go_install_bin_dir() {
  INSTALL_OPERATION="Resolve installation directory using go env GOBIN/GOPATH"
  local gobin
  gobin="$(go env GOBIN)"
  gobin="${gobin//$'\r'/}"
  gobin="${gobin//$'\n'/}"
  if [[ -n "$gobin" ]]; then
    printf '%s\n' "$gobin"
    return
  fi
  local gopath
  gopath="$(go env GOPATH)"
  printf '%s/bin\n' "${gopath%%:*}"
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
    _solomon_gopath="$(go env GOPATH)"
    _solomon_go_bin="${_solomon_gopath%%:*}/bin"
  fi
  case ":${PATH}:" in
    *":${_solomon_go_bin}:"*) ;;
    *) export PATH="${_solomon_go_bin}:${PATH}" ;;
  esac
  unset _solomon_gobin _solomon_go_bin _solomon_gopath
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
    set -l _solomon_gopath (string split ':' (go env GOPATH))
    fish_add_path --prepend $_solomon_gopath[1]/bin
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
  # BSD sort (macOS) has no -V. Compare numeric components with POSIX awk.
  awk -v have="$have" -v want="$want" 'BEGIN {
    nh=split(have,h,"."); nw=split(want,w,"."); n=nh>nw?nh:nw;
    for(i=1;i<=n;i++) { if(h[i]+0>w[i]+0) exit 0; if(h[i]+0<w[i]+0) exit 1 }
    exit 0
  }'
}

file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo 'SHA-256 verification requires sha256sum or shasum' >&2
    return 1
  fi
}

go_semver() {
  local raw
  raw="$(go version | awk '{print $3}')"
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
  INSTALL_OPERATION="Resolve latest Solomon release using $GITHUB_RELEASES_API"
  local tag api="$GITHUB_RELEASES_API"
  if command -v python3 >/dev/null 2>&1; then
    tag="$(curl -fsSL "$api" | python3 -c "import json,sys; print(json.load(sys.stdin)['tag_name'])")"
  else
    tag="$(curl -fsSL "$api" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  fi
  tag="${tag//$'\r'/}"
  tag="${tag//$'\n'/}"
  if [[ -z "$tag" ]]; then
    echo "No release tag_name found in $api. Check for API rate limiting, proxy errors or a missing published release." >&2
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

install_release_asset() (
  local platform goos goarch asset url bin_dir target tmp
  platform="$(detect_platform)"
  goos="${platform%-*}"
  goarch="${platform#*-}"
  asset="solomon-${INSTALL_VERSION}-${goos}-${goarch}"
  url="https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/${INSTALL_VERSION}/${asset}"
  bin_dir="$(go_install_bin_dir)"
  target="${bin_dir}/solomon"
  mkdir -p "$bin_dir"
  tmp="$(mktemp "$bin_dir/.solomon-download.XXXXXX")"
  checksums="$(mktemp "$bin_dir/.solomon-checksums.XXXXXX")"
  trap "rm -f $(printf '%q' "$tmp") $(printf '%q' "$checksums")" EXIT
  echo "Downloading Solomon release asset ${asset}..."
  INSTALL_OPERATION="Download Solomon asset $url to $target"
    attempt=1
  max_attempts=15
  while true; do
    if curl -fsSL "$url" -o "$tmp"; then
      break
    else
      download_status=$?
    fi
    if (( attempt >= max_attempts )); then
      echo "Failed to download ${asset} after ${max_attempts} attempts (curl exit code $download_status) from $url. See curl error above for HTTP, DNS, TLS or connection details." >&2
      rm -f "$tmp"
      exit "$download_status"
    fi
    echo "Download failed (attempt ${attempt}/${max_attempts}; curl exit code $download_status; URL: $url). See curl error above; retrying..." >&2
    sleep 2
    attempt=$((attempt + 1))
  done
  checksums_url="https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/${INSTALL_VERSION}/checksums.txt"
  INSTALL_OPERATION="Download checksum manifest $checksums_url"
  if curl -fsSL "$checksums_url" -o "$checksums"; then
    expected="$(awk -v asset="$asset" '{name=$NF; sub(/^\*/,"",name); if(name==asset) {print tolower($1); exit}}' "$checksums")"
    if [[ -z "$expected" ]]; then
      echo "checksums: no entry for ${asset} in checksums.txt" >&2
      rm -f "$tmp" "$checksums"
      exit 1
    fi
    INSTALL_OPERATION="Verify SHA-256 of $asset using $checksums_url"
    actual="$(file_sha256 "$tmp")"
    if [[ "$expected" != "$actual" ]]; then
      echo "checksum mismatch for ${asset} (expected ${expected}, got ${actual})" >&2
      rm -f "$tmp" "$checksums"
      exit 1
    fi
    rm -f "$checksums"
  else
    download_status=$?
    echo "Cannot download checksums.txt for ${INSTALL_VERSION} from $checksums_url (curl exit code $download_status); installation cancelled. See curl error above; the release must include a checksum manifest." >&2
    exit "$download_status"
  fi
  [[ -s "$tmp" ]] || { echo 'Downloaded binary is empty' >&2; exit 1; }
  # Chmod before the atomic rename; a running CLI keeps its previous inode.
  INSTALL_OPERATION="Set executable permissions and replace Solomon binary at $target"
  chmod 755 "$tmp"
  mv "$tmp" "$target"
)

install_go() (
  local platform tarball url parent tmp
  platform="$(detect_platform)"
  tarball="go${GO_REQUIRED}.${platform}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  parent="$(dirname "$GO_INSTALL_ROOT")"
  mkdir -p "$parent"
  echo "Downloading Go ${GO_REQUIRED} (${platform})..."
  tmp="$(mktemp -d "$parent/.solomon-go.XXXXXX")"
  trap 'rm -rf "$tmp"' EXIT
  INSTALL_OPERATION="Download Go from $url into $tmp"
  curl -fsSL "$url" -o "${tmp}/${tarball}"
  INSTALL_OPERATION="Extract Go archive ${tmp}/${tarball}"
  tar -C "$tmp" -xzf "${tmp}/${tarball}"
  INSTALL_OPERATION="Validate extracted Go toolchain ${tmp}/go/bin/go"
  "${tmp}/go/bin/go" version >/dev/null
  INSTALL_OPERATION="Replace Go toolchain at $GO_INSTALL_ROOT (backup and rollback enabled)"
  if [[ -e "$GO_INSTALL_ROOT" ]]; then
    mv "$GO_INSTALL_ROOT" "$tmp/previous-go"
  fi
  if ! mv "$tmp/go" "$GO_INSTALL_ROOT"; then
    if [[ -e "$tmp/previous-go" ]]; then
      if ! mv "$tmp/previous-go" "$GO_INSTALL_ROOT"; then
        trap - EXIT
        echo "Go rollback failed; previous toolchain retained in $tmp/previous-go" >&2
      fi
    fi
    exit 1
  fi
)

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
  export PATH="${GO_INSTALL_ROOT}/bin:${PATH}"
  INSTALLED_LOCAL_GO=1
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
  INSTALL_OPERATION="Install Linux packages: ${pkgs[*]} (check package-manager output and sudo permissions)"
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
        echo "No supported Linux package manager found (distribution: ${id:-unknown}; requested packages: ${pkgs[*]}). Install these packages manually, then rerun." >&2
        return 1
      fi
      ;;
  esac
}

find_homebrew() {
  local candidate
  if command -v brew >/dev/null 2>&1; then
    return 0
  fi
  for candidate in /opt/homebrew/bin/brew /usr/local/bin/brew; do
    if [[ -x "$candidate" ]]; then
      export PATH="${candidate%/brew}:${PATH}"
      return 0
    fi
  done
  return 1
}

ensure_homebrew() {
  local installer brew_prefix install_status=0
  INSTALL_OPERATION="Locate Homebrew or install from https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"
  if ! find_homebrew; then
    echo "Homebrew not found; installing from the official installer..."
    installer="$(mktemp)"
    if ! curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh -o "$installer"; then
      rm -f "$installer"
      echo "Homebrew download failed; see https://brew.sh/ and rerun this script." >&2
      return 1
    fi
    # The Solomon installer is often piped into bash: give Homebrew the
    # controlling terminal for its prompts when available.
    if ( : </dev/tty ) 2>/dev/null; then
      /bin/bash "$installer" </dev/tty || install_status=$?
    else
      NONINTERACTIVE=1 /bin/bash "$installer" || install_status=$?
    fi
    if [[ "$install_status" != 0 ]]; then
      rm -f "$installer"
      echo "Homebrew installation failed (exit code $install_status); see installer output above for administrator permissions or Xcode Command Line Tools requirements. Complete installation at https://brew.sh/ and rerun this script." >&2
      return 1
    fi
    rm -f "$installer"
    if ! find_homebrew; then
      echo "Homebrew installed but brew was not found; follow its PATH instructions and rerun this script." >&2
      return 1
    fi
  fi
  brew_prefix="$(brew --prefix)" || return 1
  BREW_BIN_DIR="${brew_prefix}/bin"
  export PATH="${BREW_BIN_DIR}:${PATH}"
  persist_github_path "$BREW_BIN_DIR"
}

install_make_darwin() {
  ensure_homebrew || return 1
  INSTALL_OPERATION="Install GNU Make with brew install make and resolve its PATH"
  echo "Installing make via Homebrew..."
  if ! brew install make; then
    echo "Homebrew make installation failed; resolve the brew error above and rerun this script." >&2
    return 1
  fi
  local brew_prefix
  brew_prefix="$(brew --prefix make)" || return 1
  MAKE_BIN_DIR="${brew_prefix}/libexec/gnubin"
  export PATH="${MAKE_BIN_DIR}:${PATH}"
  persist_github_path "$MAKE_BIN_DIR"
}

install_make_linux() {
  linux_install_packages make || {
    local status=$?
    echo "GNU Make package installation failed (exit code $status). See package-manager error above for repository, sudo, lock or package availability details; install make manually and rerun." >&2
    exit "$status"
  }
}

ensure_make() {
  if command -v make > /dev/null 2>&1; then
    INSTALL_OPERATION='Validate GNU Make using make --version'
    make --version >/dev/null
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
  INSTALL_OPERATION='Validate installed GNU Make using make --version'
  make --version >/dev/null
  echo "make ready ($(command -v make))"
}

node_lts_major() {
  local major=""
  if command -v curl >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
    major="$(curl --connect-timeout 10 --max-time 30 -fsSL https://nodejs.org/dist/index.json | python3 -c "
import json,sys
for e in json.load(sys.stdin):
    if e.get('lts'):
        print(e['version'].lstrip('v').split('.')[0])
        break
" || true)"
  fi
  if [[ -z "$major" ]]; then
    echo '[install] Warning: could not resolve Node.js LTS from https://nodejs.org/dist/index.json (curl/python3 missing or request/JSON failure). Using Node.js 22 fallback.' >&2
    major="22"
  fi
  echo "$major"
}

install_node_darwin() {
  local major formula brew_prefix
  ensure_homebrew || return 1
  major="$(node_lts_major)"
  formula="node@${major}"
  INSTALL_OPERATION="Install Node.js with brew install $formula and resolve its PATH"
  echo "Installing Node.js LTS (${formula}) via Homebrew..."
  if ! brew install "$formula"; then
    echo "${formula} installation failed; trying brew install node..."
    formula="node"
    if ! brew install "$formula"; then
      echo "Homebrew Node.js installation failed; resolve the brew error above and rerun this script." >&2
      return 1
    fi
  fi
  brew_prefix="$(brew --prefix "$formula")" || return 1
  # Versioned formulae are keg-only. Prefer the installed runtime even if an
  # older Node is already in PATH, without overwriting Homebrew's links.
  NODE_BIN_DIR="${brew_prefix}/bin"
  export PATH="${NODE_BIN_DIR}:${PATH}"
  persist_github_path "$NODE_BIN_DIR"
}

install_node_linux_fallback() {
  linux_install_packages nodejs npm || {
    local status=$?
    echo "Node.js/npm package installation failed (exit code $status). See package-manager error above for repository, sudo, lock or package availability details; install Node.js LTS and npm from https://nodejs.org/ and rerun." >&2
    exit "$status"
  }
}

install_node_linux() {
  local major
  major="$(node_lts_major)"

  if command -v curl >/dev/null 2>&1 && command -v apt-get >/dev/null 2>&1; then
    INSTALL_OPERATION="Configure NodeSource repository https://deb.nodesource.com/setup_${major}.x and install nodejs"
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
    ver="$(node --version | tr -d '\r' | sed 's/^v//' | sed 's/-.*$//' || true)"
    if version_ge "$ver" "$NODE_REQUIRED" && command -v npm >/dev/null 2>&1; then
      echo "Node ${ver} OK (>= ${NODE_REQUIRED})"
      return 0
    fi
    echo "Node ${ver:-unknown} or npm is unavailable/unsupported; installing a supported runtime..."
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
  INSTALL_OPERATION='Validate installed Node.js using node --version'
  ver="$(node --version | tr -d '\r' | sed 's/^v//' | sed 's/-.*$//' || true)"
  if ! version_ge "$ver" "$NODE_REQUIRED"; then
    echo "Node install failed (got ${ver}; need >= ${NODE_REQUIRED})" >&2
    exit 1
  fi
  if ! command -v npm >/dev/null 2>&1; then
    echo "Node installed but npm was not found in PATH" >&2
    return 1
  fi
  echo "Node ${ver} ready"
}

cloakbrowser_ready() {
  command -v node >/dev/null 2>&1 || return 1
  local node_version
  node_version="$(node --version 2>/dev/null | tr -d '\r' | sed 's/^v//' | sed 's/-.*$//')" || return 1
  version_ge "$node_version" "$NODE_REQUIRED" || return 1
  local solomon_home cloak_dir cloak_cache browser
  solomon_home="${SOLOMON_HOME:-${HOME}/.solomon}"
  cloak_dir="${solomon_home}/cloakbrowser"
  cloak_cache="${cloak_dir}/cache"
  [[ -f "${cloak_dir}/node_modules/cloakbrowser/package.json" ]] || return 1
  [[ -f "${cloak_dir}/node_modules/playwright-core/package.json" ]] || return 1
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    darwin)
      browser="$(find "$cloak_cache" -type f -path '*/Chromium.app/Contents/MacOS/Chromium' -perm -111 -print -quit 2>/dev/null || true)"
      ;;
    linux)
      browser="$(find "$cloak_cache" -type f \( -name chrome -o -name chromium -o -name chrome-headless-shell \) -perm -111 -print -quit 2>/dev/null || true)"
      ;;
    *)
      browser="$(find "$cloak_cache" -type f -name '*.exe' -perm -111 -print -quit 2>/dev/null || true)"
      ;;
  esac
  [[ -n "$browser" ]]
}

install_cloakbrowser() {
  if cloakbrowser_ready; then
    return 0
  fi
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
    INSTALL_OPERATION="Install npm packages cloakbrowser and playwright-core in $cloak_dir"
    npm install --ignore-scripts --no-audit --no-fund cloakbrowser playwright-core
    INSTALL_OPERATION="Download, verify and extract CloakBrowser into $cloak_cache"
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
    echo "SOLOMON_SERVER_PORT must be a TCP port between 1 and 65535 (received '$raw'; config: $config_path). Correct the environment variable, .env or top-level server_port setting." >&2
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
      !in_table && $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
        if (!updated) { print line; updated = 1 }
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
  INSTALL_OPERATION="Read and update MCP configuration $mcp_path"
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
  INSTALL_OPERATION="Validate server port and update runtime settings in $config_path"
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

configure_dependency_paths() {
  local rc="$1" shell_kind="$2" dir quoted line
  for dir in "$BREW_BIN_DIR" "$MAKE_BIN_DIR" "$NODE_BIN_DIR"; do
    [[ -n "$dir" ]] || continue
    printf -v quoted '%q' "$dir"
    if [[ "$shell_kind" == fish ]]; then
      line="fish_add_path --prepend $quoted"
    else
      line="export PATH=$quoted:\"\$PATH\""
    fi
    if ! line_in_file "$rc" "$line"; then
      printf '\n%s\n%s\n' "$MARKER-dependencies" "$line" >>"$rc"
    fi
  done
}

configure_unix_rc() {
  local rc go_bin
  rc="$1"
  [[ -n "$rc" ]] || return 0
  mkdir -p "$(dirname "$rc")"
  INSTALL_OPERATION="Update shell profile $rc"
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
  configure_dependency_paths "$rc" unix
}

configure_fish_rc() {
  local rc
  rc="$1"
  [[ -n "$rc" ]] || return 0
  mkdir -p "$(dirname "$rc")"
  INSTALL_OPERATION="Update shell profile $rc"
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
  configure_dependency_paths "$rc" fish
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

install_solomon() (
  resolve_install_version
  ensure_go_bin_in_path
  local server_port
  server_port="$(resolve_installer_server_port "${SOLOMON_HOME:-${HOME}/.solomon}")"
  INSTALL_OPERATION="Check for active Solomon daemon on 127.0.0.1:$server_port"
  if curl --connect-timeout 2 --max-time 3 -fsS "http://127.0.0.1:${server_port}/health" -o /dev/null 2>/dev/null; then
    echo 'A daemon is active. Use solomon upgrade for a coordinated update, or stop Solomon before reinstalling.' >&2
    return 1
  fi
  local bin_dir solomon_bin previous
  bin_dir="$(go_install_bin_dir)"
  solomon_bin="${bin_dir}/solomon"
  INSTALL_OPERATION="Back up existing Solomon CLI $solomon_bin"
  previous="$(mktemp "$bin_dir/.solomon-reinstall-backup.XXXXXX")"
  local had_previous=0
  if [[ -e "$solomon_bin" ]]; then
    cp -p "$solomon_bin" "$previous"
    had_previous=1
  fi
  rollback_release_install() {
    local status=$?
    if (( status != 0 )); then
      if (( had_previous == 1 )); then
        if ! mv -f "$previous" "$solomon_bin"; then
          echo "CLI rollback failed; original retained at $previous" >&2
          return "$status"
        fi
      else
        rm -f "$solomon_bin"
      fi
    fi
    rm -f "$previous"
    return "$status"
  }
  trap rollback_release_install EXIT
  echo "Installing solomon (${INSTALL_VERSION})..."
  INSTALL_OPERATION='Install Solomon release binary: download, checksum verification and replacement'
  install_release_asset
  if [[ -x "$solomon_bin" ]]; then
    echo "solomon installed: ${solomon_bin}"
    INSTALL_OPERATION="Provision desktop application and OS registration using $solomon_bin init"
    "$solomon_bin" init
    INSTALL_OPERATION="Validate installed CLI using $solomon_bin version"
    "$solomon_bin" version
  else
    echo "Expected executable $solomon_bin is missing or not executable after installation. Check filesystem permissions and the download/extraction errors above." >&2
    exit 1
  fi
)

setup_path_only() {
  installer_enable_diagnostics
  installer_step 'Go toolchain setup' 'Check the reported download URL, Go version, permissions and available disk space.'
  ensure_go
  installer_step 'Shell PATH and profile setup' 'Check the reported profile path and directory permissions.'
  setup_shell
}

main() {
  installer_enable_diagnostics
  if [[ "$(uname -s)" == Darwin ]]; then
    installer_step 'Homebrew setup' 'Resolve the Homebrew error above; check administrator access and Xcode Command Line Tools.'
    ensure_homebrew || return $?
  fi
  installer_step 'Go toolchain setup' 'Check the reported download URL, Go version, permissions and available disk space.'
  ensure_go
  installer_step 'GNU Make setup' 'Check the package-manager error, repository access, sudo permissions and PATH.'
  ensure_make
  installer_step 'CloakBrowser setup' 'Check Node/npm and the reported download, timeout, checksum or extraction error; check proxy and cache permissions.'
  install_cloakbrowser
  installer_step 'Shell PATH and profile setup' 'Check the reported profile path and directory permissions.'
  setup_shell
  installer_step 'Runtime configuration' 'Correct the reported port or JSON error; check configuration directory permissions.'
  configure_runtime_defaults
  installer_step 'Solomon CLI and desktop setup' 'Check the reported release URL, checksum, active daemon or desktop setup error. Use solomon upgrade if Solomon is running.'
  install_solomon
  echo "Done."
}

if [[ "${1:-}" == "--setup-path-only" ]]; then
  setup_path_only
  exit 0
fi

if [[ "${BASH_SOURCE[0]:-${0}}" == "${0}" ]]; then
  main "$@"
fi
