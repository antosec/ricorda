// Package index maintains a small, fast lookup of certified victories per
// tool. It exists so the post-failure whisper can answer in milliseconds
// without ever reading the full journal.
package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/antosec/ricorda/internal/analyze"
	"github.com/antosec/ricorda/internal/paths"
)

// Victory is one certified hard-won command, ready to be suggested.
type Victory struct {
	Cmd      string    `json:"cmd"`
	Attempts int       `json:"attempts"`
	CostMS   int64     `json:"cost_ms"`
	CWD      string    `json:"cwd,omitempty"`
	TS       time.Time `json:"ts"`
}

// File is the on-disk shape of the index.
type File struct {
	Version int                  `json:"version"`
	Tools   map[string][]Victory `json:"tools"`
}

const maxPerTool = 8

// Path returns the index location.
func Path() (string, error) {
	h, err := paths.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "index.json"), nil
}

// Write rebuilds the index from analyzed reports, keeping only certified
// victories (they carry ground truth worth whispering).
func Write(reports []analyze.ToolReport) error {
	f := File{Version: 1, Tools: map[string][]Victory{}}
	for _, r := range reports {
		var vs []Victory
		for _, h := range r.HardWon {
			if !h.Certified {
				continue
			}
			vs = append(vs, Victory{
				Cmd:      h.Command,
				Attempts: h.Attempts,
				CostMS:   h.CostMS,
				CWD:      h.CWD,
				TS:       h.TS,
			})
			if len(vs) >= maxPerTool {
				break
			}
		}
		if len(vs) > 0 {
			f.Tools[r.Tool] = vs
		}
	}

	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// Read loads the index; a missing file is an empty index, never an error.
func Read() (File, error) {
	f := File{Version: 1, Tools: map[string][]Victory{}}
	p, err := Path()
	if err != nil {
		return f, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		// A corrupt index must never break a prompt hook; start clean.
		return File{Version: 1, Tools: map[string][]Victory{}}, nil
	}
	if f.Tools == nil {
		f.Tools = map[string][]Victory{}
	}
	return f, nil
}
