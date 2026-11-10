package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
	toml "github.com/pelletier/go-toml/v2"
)

type ProgrammingLanguage struct {
	Name                  string         `toml:"name" json:"name"`
	FirstUsed             string         `toml:"first_used" json:"firstUsed"`
	LastUsed              string         `toml:"last_used" json:"lastUsed"`
	Mode                  string         `toml:"mode" json:"mode"`
	Command               []string       `toml:"command,omitempty" json:"command"`
	Settings              map[string]any `toml:"settings,omitempty" json:"settings"`
	InitializationOptions map[string]any `toml:"initialization_options,omitempty" json:"initializationOptions"`
	Completion            *bool          `toml:"completion,omitempty" json:"completion,omitempty"`
	Hover                 *bool          `toml:"hover,omitempty" json:"hover,omitempty"`
	Diagnostics           *bool          `toml:"diagnostics,omitempty" json:"diagnostics,omitempty"`
}

type ProgrammingLanguages map[string]ProgrammingLanguage

var programmingLanguageID = regexp.MustCompile(`^[a-z0-9][a-z0-9_.+-]{0,95}$`)

func (p ProgrammingLanguage) Validate(id string) error {
	if !programmingLanguageID.MatchString(id) {
		return errors.New("invalid programming language identifier")
	}
	if p.Mode != "auto" && p.Mode != "custom" && p.Mode != "fallback" {
		return errors.New("language server mode must be auto, custom or fallback")
	}
	if len(p.Name) == 0 || len(p.Name) > 128 {
		return errors.New("language name must contain 1 to 128 bytes")
	}
	if len(p.Command) > 64 {
		return errors.New("language server command has too many arguments")
	}
	for _, arg := range p.Command {
		if len(arg) > 4096 || regexp.MustCompile(`[\x00\r\n]`).MatchString(arg) {
			return errors.New("invalid language server argument")
		}
	}
	if p.Mode == "custom" && (len(p.Command) == 0 || p.Command[0] == "") {
		return errors.New("a custom language server requires an executable")
	}
	data, err := json.Marshal(p)
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("language settings exceed 64 KiB or contain invalid values")
	}
	return nil
}

func ReadProgrammingLanguages() (ProgrammingLanguages, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ProgrammingLanguages{}, nil
	}
	if err != nil {
		return nil, err
	}
	var file struct {
		Languages ProgrammingLanguages `toml:"programming_languages"`
	}
	if err = toml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.Languages == nil {
		file.Languages = ProgrammingLanguages{}
	}
	return file.Languages, nil
}

func UpdateProgrammingLanguage(id string, value ProgrammingLanguage, used bool) (ProgrammingLanguage, error) {
	if value.Mode == "" {
		value.Mode = "auto"
	}
	if err := value.Validate(id); err != nil {
		return value, err
	}
	path, err := paths.ConfigPath()
	if err != nil {
		return value, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return value, err
	}
	lock := flock.New(filepath.Join(filepath.Dir(path), "config.lock"))
	if err = lock.Lock(); err != nil {
		return value, err
	}
	defer lock.Unlock()
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return value, err
	}
	document := map[string]any{}
	if len(data) > 0 {
		if err = toml.Unmarshal(data, &document); err != nil {
			return value, err
		}
	}
	languages, err := ReadProgrammingLanguages()
	if err != nil {
		return value, err
	}
	previous, exists := languages[id]
	if used && exists {
		value = previous
	}
	now := time.Now().UTC().Format(time.RFC3339)
	value.FirstUsed = previous.FirstUsed
	if value.FirstUsed == "" {
		value.FirstUsed = now
	}
	value.LastUsed = previous.LastUsed
	if used || value.LastUsed == "" {
		value.LastUsed = now
	}
	languages[id] = value
	document["programming_languages"] = languages
	encoded, err := toml.Marshal(document)
	if err != nil {
		return value, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".languages-*")
	if err != nil {
		return value, err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(encoded); err != nil {
		temporary.Close()
		return value, err
	}
	if err = temporary.Close(); err != nil {
		return value, err
	}
	return value, os.Rename(temporary.Name(), path)
}

func programmingLanguagesFingerprint(value ProgrammingLanguages) string {
	data, _ := json.Marshal(value)
	return string(data)
}
