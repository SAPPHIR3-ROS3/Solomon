// Package sidecar ships the Claude Sub sidecar sources inside Solomon binaries,
// so installed clients can run it without a repository checkout.
package sidecar

import "embed"

//go:embed package.json package-lock.json src/*.ts
var Files embed.FS
