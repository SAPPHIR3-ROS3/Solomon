//go:build automatic_role_scores

package benchmarks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PublishPaths struct {
	ExtraScores   string
	ExtraManifest string
	EmbedScores   string
	EmbedManifest string
}

func PublishScores(scores ScoresFile, manifest Manifest, paths PublishPaths) error {
	if err := ValidateScoresFile(scores); err != nil {
		return err
	}
	outScores, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(outScores)
	if strings.TrimSpace(manifest.ScoresSHA256) == "" {
		manifest.ScoresSHA256 = hex.EncodeToString(sum[:])
	}
	outManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if paths.ExtraScores != "" {
		if err := os.MkdirAll(filepath.Dir(paths.ExtraScores), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(paths.ExtraScores, outScores, 0o644); err != nil {
			return err
		}
	}
	if paths.ExtraManifest != "" {
		if err := os.WriteFile(paths.ExtraManifest, outManifest, 0o644); err != nil {
			return err
		}
	}
	if paths.EmbedScores != "" {
		if err := os.WriteFile(paths.EmbedScores, outScores, 0o644); err != nil {
			return err
		}
	}
	if paths.EmbedManifest != "" {
		if err := os.WriteFile(paths.EmbedManifest, outManifest, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func VerifyScoresSHA(data []byte, manifest Manifest) error {
	want := strings.TrimSpace(manifest.ScoresSHA256)
	if want == "" {
		return nil
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("scores checksum mismatch (want %s, got %s)", want, got)
	}
	return nil
}
