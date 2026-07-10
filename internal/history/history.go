// Package history discovers and parses shell history files.
package history

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Entry is a single command as typed by the user, in original order.
type Entry struct {
	Command string
	Source  string
}

// SourceResult is the outcome of probing one history location.
type SourceResult struct {
	Name    string
	Path    string
	Entries []Entry
	Err     error
}

// Load probes every known history location and parses the ones that exist.
// A missing or unreadable source is recorded in Err, never fatal.
func Load() []SourceResult {
	var out []SourceResult
	for _, p := range probes() {
		res := SourceResult{Name: p.name, Path: p.path}
		f, err := os.Open(p.path)
		if err != nil {
			res.Err = err
			out = append(out, res)
			continue
		}
		entries, err := p.parse(f)
		f.Close()
		if err != nil {
			res.Err = err
		}
		for i := range entries {
			entries[i].Source = p.name
		}
		res.Entries = entries
		out = append(out, res)
	}
	return out
}

type probe struct {
	name  string
	path  string
	parse func(io.Reader) ([]Entry, error)
}

func probes() []probe {
	var ps []probe
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		ps = append(ps, probe{
			name:  "pwsh",
			path:  filepath.Join(appdata, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"),
			parse: ParsePwsh,
		})
	}
	if home, err := os.UserHomeDir(); err == nil {
		ps = append(ps,
			probe{name: "bash", path: filepath.Join(home, ".bash_history"), parse: ParseBash},
			probe{name: "zsh", path: filepath.Join(home, ".zsh_history"), parse: ParseZsh},
			probe{name: "fish", path: filepath.Join(home, ".local", "share", "fish", "fish_history"), parse: ParseFish},
		)
	}
	return ps
}

// ParseBash reads plain bash history: one command per line. Epoch timestamp
// lines written when HISTTIMEFORMAT is set ("#1720000000") are skipped.
func ParseBash(r io.Reader) ([]Entry, error) {
	var out []Entry
	sc := scanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || isBashTimestamp(line) {
			continue
		}
		out = append(out, Entry{Command: line})
	}
	return out, sc.Err()
}

func isBashTimestamp(line string) bool {
	if !strings.HasPrefix(line, "#") || len(line) == 1 {
		return false
	}
	for _, c := range line[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ParseZsh handles both plain and extended (": <ts>:<elapsed>;cmd") formats.
func ParseZsh(r io.Reader) ([]Entry, error) {
	var out []Entry
	sc := scanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, ": ") {
			i := strings.Index(line, ";")
			if i < 0 {
				continue
			}
			line = strings.TrimSpace(line[i+1:])
		}
		if line != "" {
			out = append(out, Entry{Command: line})
		}
	}
	return out, sc.Err()
}

// ParseFish extracts commands from fish's YAML-like history file.
func ParseFish(r io.Reader) ([]Entry, error) {
	var out []Entry
	sc := scanner(r)
	for sc.Scan() {
		if cmd, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "- cmd: "); ok {
			cmd = strings.TrimSpace(cmd)
			if cmd != "" {
				out = append(out, Entry{Command: cmd})
			}
		}
	}
	return out, sc.Err()
}

// ParsePwsh reads PSReadLine's ConsoleHost_history.txt. Multi-line commands
// are stored with a trailing backtick on every line but the last; they are
// joined back into one logical command.
func ParsePwsh(r io.Reader) ([]Entry, error) {
	var out []Entry
	var cont strings.Builder
	sc := scanner(r)
	flush := func() {
		cmd := strings.TrimSpace(cont.String())
		cont.Reset()
		if cmd != "" {
			out = append(out, Entry{Command: cmd})
		}
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.HasSuffix(line, "`") {
			cont.WriteString(strings.TrimSpace(strings.TrimSuffix(line, "`")))
			cont.WriteString(" ")
			continue
		}
		cont.WriteString(strings.TrimSpace(line))
		flush()
	}
	flush()
	return out, sc.Err()
}

// scanner returns a line scanner with a generous buffer for long commands.
func scanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return sc
}
