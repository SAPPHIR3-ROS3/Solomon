package cursor

import "embed"

//go:embed bundle/dist bundle/package.json bundle/package-lock.json bundle/.npmrc
var bundleFS embed.FS
