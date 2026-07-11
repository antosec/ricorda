// Package hook installs the ricorda capture block into shell startup files
// and removes it again. Blocks are delimited by markers so installs are
// idempotent, upgrades replace in place, and uninstalls are surgical.
package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// BeginMark and EndMark delimit the managed block in rc files.
	BeginMark = "# >>> ricorda hook >>>"
	EndMark   = "# <<< ricorda hook <<<"
)

// Target is one shell startup file we can manage.
type Target struct {
	Shell   string
	Path    string
	OwnFile bool // the whole file belongs to ricorda (fish conf.d)
}

// Upsert inserts the managed block into content, or replaces the existing
// one in place.
func Upsert(content, block string) string {
	block = strings.TrimSpace(block)
	if start, end, ok := findBlock(content); ok {
		return content[:start] + block + content[end:]
	}
	trimmed := strings.TrimRight(content, " \t\r\n")
	if trimmed == "" {
		return block + "\n"
	}
	return trimmed + "\n\n" + block + "\n"
}

// Remove deletes the managed block; it reports whether anything changed.
func Remove(content string) (string, bool) {
	start, end, ok := findBlock(content)
	if !ok {
		return content, false
	}
	before := strings.TrimRight(content[:start], " \t")
	after := content[end:]
	after = strings.TrimPrefix(after, "\r\n")
	after = strings.TrimPrefix(after, "\n")
	return before + after, true
}

func findBlock(content string) (start, end int, ok bool) {
	start = strings.Index(content, BeginMark)
	if start < 0 {
		return 0, 0, false
	}
	rel := strings.Index(content[start:], EndMark)
	if rel < 0 {
		// Malformed (begin without end): treat as absent rather than risk
		// eating user content.
		return 0, 0, false
	}
	return start, start + rel + len(EndMark), true
}

// InstallFile writes the block into path, creating parent directories and
// preserving everything else in the file.
func InstallFile(path, block string, ownFile bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if ownFile {
		return os.WriteFile(path, []byte(strings.TrimSpace(block)+"\n"), 0o644)
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(Upsert(string(b), block)), filePerm(path))
}

// UninstallFile removes the block (or the whole file when ownFile); it
// reports whether anything was removed.
func UninstallFile(path string, ownFile bool) (bool, error) {
	if ownFile {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	out, changed := Remove(string(b))
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(out), filePerm(path))
}

// Installed reports whether path currently contains the managed block.
func Installed(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, _, ok := findBlock(string(b))
	return ok
}

func filePerm(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

// DetectTargets returns the startup file of every shell present on this
// machine. PowerShell profiles are asked to the shells themselves, since
// Documents folders can be relocated.
func DetectTargets() []Target {
	var ts []Target
	for _, bin := range []string{"powershell", "pwsh"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		out, err := exec.Command(bin, "-NoProfile", "-NonInteractive", "-Command", "Write-Output $PROFILE").Output()
		if err != nil {
			continue
		}
		p := strings.TrimSpace(string(out))
		if p != "" && !hasPath(ts, p) {
			ts = append(ts, Target{Shell: "pwsh", Path: p})
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ts
	}
	bashRC := filepath.Join(home, ".bashrc")
	if lookPathOK("bash") || exists(bashRC) {
		ts = append(ts, Target{Shell: "bash", Path: bashRC})
	}
	zdot := os.Getenv("ZDOTDIR")
	if zdot == "" {
		zdot = home
	}
	zshRC := filepath.Join(zdot, ".zshrc")
	if lookPathOK("zsh") || exists(zshRC) {
		ts = append(ts, Target{Shell: "zsh", Path: zshRC})
	}
	fishDir := filepath.Join(home, ".config", "fish")
	if lookPathOK("fish") || exists(fishDir) {
		ts = append(ts, Target{Shell: "fish", Path: filepath.Join(fishDir, "conf.d", "ricorda.fish"), OwnFile: true})
	}
	return ts
}

func lookPathOK(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func hasPath(ts []Target, p string) bool {
	for _, t := range ts {
		if t.Path == p {
			return true
		}
	}
	return false
}
