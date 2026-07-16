//go:build automatic_role_scores

package benchmarks

import (
	_ "embed"
)

//go:embed scores.json
var embeddedScoresJSON []byte

//go:embed manifest.json
var embeddedManifestJSON []byte

func loadEmbedded() (*Store, error) {
	scores, err := ParseScoresJSON(embeddedScoresJSON)
	if err != nil {
		return nil, err
	}
	manifest, err := ParseManifestJSON(embeddedManifestJSON)
	if err != nil {
		return nil, err
	}
	if err := ValidateScoresFile(scores); err != nil {
		return nil, err
	}
	return &Store{Scores: scores, Manifest: manifest, Source: "embedded"}, nil
}

func EmbeddedScoresJSON() []byte {
	return append([]byte(nil), embeddedScoresJSON...)
}

func EmbeddedManifestJSON() []byte {
	return append([]byte(nil), embeddedManifestJSON...)
}
