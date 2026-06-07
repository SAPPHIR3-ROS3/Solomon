package docs

import "embed"

//go:embed README.md features.md architecture development user-guide
var Corpus embed.FS
