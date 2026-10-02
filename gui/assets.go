// Package gui provides the production frontend shared by the web and desktop clients.
package gui

import (
	"embed"
	"io/fs"
)

//go:embed all:assets/frontend
var assets embed.FS

func Assets() fs.FS {
	frontend, err := fs.Sub(assets, "assets/frontend")
	if err != nil {
		panic(err)
	}
	return frontend
}

func Ready() bool {
	_, err := fs.Stat(Assets(), "index.html")
	return err == nil
}
