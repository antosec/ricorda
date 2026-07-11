// Package paths resolves ricorda's on-disk locations.
package paths

import (
	"os"
	"path/filepath"
)

// Home returns the ricorda home directory ($RICORDA_HOME or ~/.ricorda).
func Home() (string, error) {
	if d := os.Getenv("RICORDA_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ricorda"), nil
}
