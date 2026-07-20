//go:build automatic_role_scores

package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

func normalizeModelID(id string) string {
	return roles.NormalizeModelID(id)
}

type Manifest struct {
	GeneratedAt  string `json:"generated_at"`
	ScoresSHA256 string `json:"scores_sha256,omitempty"`
	Commit       string `json:"commit,omitempty"`
}

type ScoresFile struct {
	Version     int                       `json:"version"`
	GeneratedAt string                    `json:"generated_at"`
	Sources     []string                  `json:"sources,omitempty"`
	Models      map[string]map[string]int `json:"models"`
}

type Store struct {
	Scores   ScoresFile
	Manifest Manifest
	Source   string
}

func ExtraDir() (string, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "extra"), nil
}

func ScoresPath() (string, error) {
	dir, err := ExtraDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "scores.json"), nil
}

func ManifestPath() (string, error) {
	dir, err := ExtraDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manifest.json"), nil
}

func EnsureExtraDir() error {
	dir, err := ExtraDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o700)
}

func LoadStore() (*Store, error) {
	disk, diskErr := loadFromDisk()
	emb, embErr := loadEmbedded()
	if diskErr != nil && embErr != nil {
		return nil, diskErr
	}
	if diskErr != nil {
		return emb, nil
	}
	if embErr != nil {
		return disk, nil
	}
	return pickNewerStore(disk, emb), nil
}

func pickNewerStore(disk, emb *Store) *Store {
	if disk == nil {
		return emb
	}
	if emb == nil {
		return disk
	}
	td, okd := ManifestTime(disk.Manifest)
	te, oke := ManifestTime(emb.Manifest)
	if !okd && !oke {
		if len(emb.Scores.Models) > len(disk.Scores.Models) {
			return emb
		}
		return disk
	}
	if !okd {
		return emb
	}
	if !oke {
		return disk
	}
	if te.After(td) {
		return emb
	}
	if td.After(te) {
		return disk
	}
	if len(emb.Scores.Models) > len(disk.Scores.Models) {
		return emb
	}
	return disk
}

func loadFromDisk() (*Store, error) {
	scoresPath, err := ScoresPath()
	if err != nil {
		return nil, err
	}
	manifestPath, err := ManifestPath()
	if err != nil {
		return nil, err
	}
	scoresData, err := os.ReadFile(scoresPath)
	if err != nil {
		return nil, err
	}
	var scores ScoresFile
	if err := json.Unmarshal(scoresData, &scores); err != nil {
		return nil, err
	}
	var manifest Manifest
	if manifestData, mErr := os.ReadFile(manifestPath); mErr == nil {
		_ = json.Unmarshal(manifestData, &manifest)
	}
	if err := ValidateScoresFile(scores); err != nil {
		return nil, err
	}
	return &Store{Scores: scores, Manifest: manifest, Source: "disk"}, nil
}

func WriteStore(scores ScoresFile, manifest Manifest) error {
	if err := EnsureExtraDir(); err != nil {
		return err
	}
	scoresPath, err := ScoresPath()
	if err != nil {
		return err
	}
	manifestPath, err := ManifestPath()
	if err != nil {
		return err
	}
	scoresData, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(scoresPath, scoresData, 0o600); err != nil {
		return err
	}
	return os.WriteFile(manifestPath, manifestData, 0o600)
}

func (s *Store) LookupModel(modelID string) (map[string]int, bool) {
	if s == nil || len(s.Scores.Models) == 0 {
		return nil, false
	}
	key := normalizeModelID(modelID)
	if key == "" {
		return nil, false
	}
	if m, ok := s.lookupKey(key); ok {
		return m, true
	}
	for {
		i := strings.LastIndex(key, "-")
		if i < 0 {
			break
		}
		key = key[:i]
		if m, ok := s.lookupKey(key); ok {
			return m, true
		}
	}
	query := normalizeModelID(modelID)
	bestKey := ""
	for k := range s.Scores.Models {
		nk := normalizeModelID(k)
		if nk == query || strings.HasPrefix(nk, query+"-") {
			if bestKey == "" || roles.CompareModelRecency(nk, bestKey) > 0 {
				bestKey = nk
			}
		}
	}
	if bestKey != "" {
		return s.lookupKey(bestKey)
	}
	return nil, false
}

func (s *Store) lookupKey(key string) (map[string]int, bool) {
	if key == "" {
		return nil, false
	}
	if m, ok := s.Scores.Models[key]; ok {
		return m, true
	}
	for k, v := range s.Scores.Models {
		if normalizeModelID(k) == key {
			return v, true
		}
	}
	return nil, false
}

func (s *Store) LookupCharacteristic(modelID, ch string) (int, bool) {
	m, ok := s.LookupModel(modelID)
	if !ok {
		return 0, false
	}
	v, ok := m[ch]
	return v, ok
}

func ParseScoresJSON(data []byte) (ScoresFile, error) {
	var scores ScoresFile
	if err := json.Unmarshal(data, &scores); err != nil {
		return ScoresFile{}, err
	}
	if scores.Models == nil {
		scores.Models = map[string]map[string]int{}
	}
	return scores, nil
}

func ParseManifestJSON(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ManifestTime(m Manifest) (time.Time, bool) {
	raw := strings.TrimSpace(m.GeneratedAt)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func ValidateScoresFile(s ScoresFile) error {
	if s.Version <= 0 {
		return fmt.Errorf("scores.json: missing version")
	}
	if len(s.Models) == 0 {
		return fmt.Errorf("scores.json: empty models")
	}
	for modelID, scores := range s.Models {
		if strings.TrimSpace(modelID) == "" {
			return fmt.Errorf("scores.json: empty model id")
		}
		for characteristic, score := range scores {
			if !roles.IsKnownCharacteristic(characteristic) {
				return fmt.Errorf("scores.json: unknown characteristic %q for model %q", characteristic, modelID)
			}
			if err := roles.ValidateScoreValue(characteristic, score); err != nil {
				return fmt.Errorf("scores.json: model %q: %w", modelID, err)
			}
		}
	}
	return nil
}
