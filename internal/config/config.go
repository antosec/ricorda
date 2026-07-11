// Package config stores ricorda's few user preferences in a plain JSON
// file the user can read and edit.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/antosec/ricorda/internal/paths"
)

func file() (string, error) {
	h, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "config.json"), nil
}

// Load returns all settings; a missing file is an empty config.
func Load() map[string]string {
	out := map[string]string{}
	p, err := file()
	if err != nil {
		return out
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return out
}

// Set persists one setting.
func Set(key, value string) error {
	cfg := Load()
	cfg[key] = value
	p, err := file()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// WhisperEnabled reports whether post-failure suggestions are on. They
// default to on; `ricorda config whisper off` silences them.
func WhisperEnabled() bool {
	return Load()["whisper"] != "off"
}
