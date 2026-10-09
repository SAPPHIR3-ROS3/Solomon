#!/usr/bin/env bash
set -euo pipefail

# Compile only the committed source that consumers receive, with no bundlers.
source_root="$(mktemp -d)"
trap 'rm -rf "$source_root"' EXIT
git archive "${1:-HEAD}" | tar -x -C "$source_root"

for asset in internal/integrations/cursor/bundle/dist/index.js \
  internal/integrations/cursor/bundle/package.json \
  internal/integrations/cursor/bundle/package-lock.json \
  internal/integrations/cursor/bundle/.npmrc gui/assets/frontend/index.html; do
  if [[ ! -s "$source_root/$asset" ]]; then
    echo "Release source is missing embedded asset: $asset" >&2
    exit 1
  fi
done

for platform in linux windows darwin; do
  echo "Checking release source: $platform/amd64"
  (
    cd "$source_root"
    CGO_ENABLED=0 GOOS="$platform" GOARCH=amd64 \
      go build -trimpath -o "$source_root/check-$platform" ./cmd/solomon
  )
done
