package cursor

import "embed"

//go:embed bundle/dist bundle/package.json bundle/package-lock.json
var bundleFS embed.FS
