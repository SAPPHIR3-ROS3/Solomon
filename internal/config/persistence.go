package config

import (
	"bytes"
	"context"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
	to "github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"strings"
)

func normalizeRoot(r *Root) {

	if r == nil {

		return

	}

	for name, p := range r.Providers {

		if p == nil {

			delete(r.Providers, name)

			continue

		}

		if strings.TrimSpace(p.Name) == "" {

			p.Name = name

		}

	}

	r.Tools.CursorInternalTools = false
	migrateMisnamedCursorSub(r)
}

func validateRoot(ctx context.Context, r *Root) error {
	if r == nil {
		return nil
	}
	if err := ValidateServerPort(r.ServerPort); err != nil {
		return err
	}
	// Loading and saving preferences must not depend on provider network calls.
	return validateRoles(ctx, r, false)
}

func Load() (*Root, error) {

	cfgPath, err := paths.ConfigPath()

	if err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config path resolve failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})

		return nil, err

	}

	b, err := os.ReadFile(cfgPath)

	if err != nil {

		if !os.IsNotExist(err) {

			logging.Log(logging.ERROR_LOG_LEVEL, "config read failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

		}

		return nil, err

	}

	if bytes.Contains(b, []byte("[[providers]]")) {

		var leg rootLegacyFile

		if err := to.Unmarshal(b, &leg); err != nil {

			logging.Log(logging.ERROR_LOG_LEVEL, "config unmarshal legacy failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

			return nil, err

		}

		r := rootFromLegacy(&leg)

		normalizeRoot(r)
		if err := LoadModelVisibility(r); err != nil {
			return nil, err
		}

		if err := validateRoot(context.Background(), r); err != nil {
			return nil, err
		}

		return r, nil

	}

	var f rootFile

	if err := to.Unmarshal(b, &f); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config unmarshal failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

		return nil, err

	}

	r := rootFromFile(&f)

	if len(r.Providers) == 0 {

		var leg rootLegacyFile

		if err := to.Unmarshal(b, &leg); err == nil && len(leg.Providers) > 0 {

			r = rootFromLegacy(&leg)

		}

	}

	if len(r.RecentModels) == 0 && bytes.Contains(b, []byte("[[recent_model_uses]]")) {

		var leg rootLegacyFile

		if err := to.Unmarshal(b, &leg); err == nil {

			for i := len(leg.RecentModelUses) - 1; i >= 0; i-- {

				u := leg.RecentModelUses[i]

				NoteRecentModelUse(r, u.Provider, u.Model)

			}

		}

	}

	normalizeRoot(r)
	if err := LoadModelVisibility(r); err != nil {
		return nil, err
	}

	if err := validateRoot(context.Background(), r); err != nil {
		return nil, err
	}

	return r, nil

}

func Save(r *Root) error {
	path, err := paths.ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(filepath.Dir(path), "config.lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	return saveUnlocked(r)
}

func saveUnlocked(r *Root) error {
	if r != nil && (r.programmingLanguagesSnapshot == programmingLanguagesFingerprint(r.ProgrammingLanguages) || r.ProgrammingLanguages == nil) {
		latest, err := ReadProgrammingLanguages()
		if err != nil {
			return err
		}
		r.ProgrammingLanguages = latest
	}
	// Preserve GUI preferences from newer targeted writes when saving a stale
	// runtime snapshot. Explicit changes to GUI on the Root are still respected.
	if r != nil && (r.guiSnapshot == guiFingerprint(r.GUI) || (r.guiSnapshot == "" && guiFingerprint(r.GUI) == guiFingerprint(GUISettings{}))) {
		latest, err := ReadGUISettings()
		if err == nil {
			r.GUI = latest
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	normalizeRoot(r)

	if err := validateRoot(context.Background(), r); err != nil {
		return err
	}

	cfgPath, err := paths.ConfigPath()

	if err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config path resolve failed on save", logging.LogOptions{Params: map[string]any{"err": err.Error()}})

		return err

	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config mkdir failed", logging.LogOptions{Params: map[string]any{"path": filepath.Dir(cfgPath), "err": err.Error()}})

		return err

	}

	buf, err := to.Marshal(rootToFile(r))

	if err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config marshal failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})

		return err

	}

	tmp := cfgPath + ".tmp"

	if err := os.WriteFile(tmp, buf, 0o600); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config write temp failed", logging.LogOptions{Params: map[string]any{"path": tmp, "err": err.Error()}})

		return err

	}

	if err := os.Rename(tmp, cfgPath); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config rename failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

		return err

	}

	r.guiSnapshot = guiFingerprint(r.GUI)
	r.programmingLanguagesSnapshot = programmingLanguagesFingerprint(r.ProgrammingLanguages)
	return nil

}
