package gui

import (
	"embed"
	"io/fs"
)

// Sources travel with the installed binary so customization needs no repository
// checkout and always starts from the version that the user is running.
//
//go:embed src scripts wailsjs *.ts package.json package-lock.json tsconfig.json index.html
var sources embed.FS

func Sources() fs.FS { return sources }
