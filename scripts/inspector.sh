#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
export SOLOMON_WEB_INSPECTOR=1
exec "$root/gui/desktop/build/bin/solomon-inspector" "$@"
