#!/usr/bin/env bash
# Install the native Linux client and its application-menu entry for this user.
set -euo pipefail
[[ "$(uname -s)" == Linux ]] || { echo 'desktop-install currently supports Linux' >&2; exit 1; }
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_dir="${1:-$(go env GOBIN)}"
[[ -n "$bin_dir" ]] || bin_dir="$(go env GOPATH)/bin"
mkdir -p "$bin_dir"
bin_dir="$(cd "$bin_dir" && pwd)"
[[ -x "$bin_dir/solomon" ]] || { echo "Install the Solomon CLI in $bin_dir first" >&2; exit 1; }
temporary="$(mktemp "$bin_dir/.solomon-desktop.XXXXXX")"
trap 'rm -f "$temporary"' EXIT
install -m 755 "$repo_dir/gui/desktop/build/bin/solomon-desktop" "$temporary"
mv -f "$temporary" "$bin_dir/solomon-desktop"
data_dir="${XDG_DATA_HOME:-$HOME/.local/share}"
mkdir -p "$data_dir/applications" "$data_dir/icons/hicolor/scalable/apps"
install -m 644 "$repo_dir/icon.svg" "$data_dir/icons/hicolor/scalable/apps/solomon.svg"
# Desktop Exec has its own quoting rules, independent of shell quoting.
exec_path="${bin_dir//\\/\\\\}"
exec_path="${exec_path//\"/\\\"}"
exec_path="${exec_path//\$/\\\$}"
exec_path="${exec_path//\`/\\\`}"
exec_path="${exec_path//%/%%}"
cat > "$data_dir/applications/solomon.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Solomon
Comment=Solomon AI workspace
Exec="$exec_path/solomon-desktop"
Icon=solomon
Terminal=false
Categories=Development;
StartupWMClass=solomon
DESKTOP
chmod 644 "$data_dir/applications/solomon.desktop"
if command -v desktop-file-validate >/dev/null; then desktop-file-validate "$data_dir/applications/solomon.desktop"; fi
if command -v update-desktop-database >/dev/null; then update-desktop-database "$data_dir/applications"; fi
if command -v gtk-update-icon-cache >/dev/null; then gtk-update-icon-cache -f -t "$data_dir/icons/hicolor" >/dev/null 2>&1 || true; fi
printf 'Solomon desktop installed: %s\nApplication entry: %s\n' "$bin_dir/solomon-desktop" "$data_dir/applications/solomon.desktop"
