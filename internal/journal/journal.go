// Package journal stores ground-truth command records captured by the shell
// hooks: what ran, whether it worked, how long it took and where. Records
// are redacted before they are written, live in monthly JSONL files under
// the ricorda home, and never leave the machine.
package journal

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/antosec/ricorda/internal/paths"
	"github.com/antosec/ricorda/internal/redact"
)

// Entry is one executed command with its outcome.
type Entry struct {
	TS    time.Time `json:"ts"`
	Cmd   string    `json:"cmd"`
	Exit  int       `json:"exit"`
	DurMS int64     `json:"dur_ms"`
	CWD   string    `json:"cwd,omitempty"`
	Shell string    `json:"shell,omitempty"`
}

// maxCmdLen caps a single recorded command; anything longer is truncated.
const maxCmdLen = 8 * 1024

// Dir returns the journal directory.
func Dir() (string, error) {
	h, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "journal"), nil
}

// Append writes one entry to the current month's file. The command is
// trimmed, truncated and redacted before it touches disk; empty commands
// are silently dropped.
func Append(e Entry) error {
	if e.TS.IsZero() {
		e.TS = time.Now()
	}
	e.Cmd = strings.TrimSpace(e.Cmd)
	if e.Cmd == "" {
		return nil
	}
	if len(e.Cmd) > maxCmdLen {
		e.Cmd = e.Cmd[:maxCmdLen]
	}
	e.Cmd = redact.Clean(e.Cmd)

	d, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := filepath.Join(d, e.TS.Format("2006-01")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// ReadAll returns every journal entry in chronological order. Unparseable
// lines are skipped: a corrupt record must never break analysis.
func ReadAll() ([]Entry, error) {
	d, err := Dir()
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, de := range dirEntries {
		if !de.IsDir() && strings.HasSuffix(de.Name(), ".jsonl") {
			names = append(names, de.Name())
		}
	}
	sort.Strings(names) // YYYY-MM sorts chronologically

	var out []Entry
	for _, name := range names {
		f, err := os.Open(filepath.Join(d, name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), maxCmdLen+4096)
		for sc.Scan() {
			var e Entry
			if json.Unmarshal(sc.Bytes(), &e) == nil && e.Cmd != "" {
				out = append(out, e)
			}
		}
		f.Close()
	}
	return out, nil
}

// DecodeB64 decodes the base64 command payload used by the shell hooks to
// sidestep cross-shell quoting.
func DecodeB64(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("invalid base64 command payload: %w", err)
	}
	return string(b), nil
}
