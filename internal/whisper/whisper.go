// Package whisper decides what to say — and whether to say anything — right
// after a command fails. It must stay fast enough to run synchronously in a
// prompt hook, so it only ever touches the small index and a state file.
package whisper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/antosec/ricorda/internal/index"
	"github.com/antosec/ricorda/internal/project"
)

// Cooldown is how long a tool stays quiet after one whisper. Repeated
// failures during a fight should not repeat the same advice.
const Cooldown = time.Hour

// Suggestion is a past victory worth showing for a fresh failure.
type Suggestion struct {
	Tool     string
	Cmd      string
	Attempts int
	CostMS   int64
	When     time.Time
}

// Pick returns the best past victory for a failed command, or nil when
// silence is better: unknown tool, no victories, or the failed command IS
// the stored victory (suggesting what just failed would be cruel).
func Pick(idx index.File, tool, failedCmd, cwd string) *Suggestion {
	if tool == "" {
		return nil
	}
	vs := idx.Tools[tool]
	if len(vs) == 0 {
		return nil
	}

	best := -1
	for i, v := range vs {
		if v.Cmd == failedCmd {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		// Same-project victories beat global ones; ties go to the harder
		// fight (more attempts = more worth remembering).
		bi, vi := vs[best], v
		biSame := cwd != "" && project.Same(bi.CWD, cwd)
		viSame := cwd != "" && project.Same(vi.CWD, cwd)
		if viSame != biSame {
			if viSame {
				best = i
			}
			continue
		}
		if vi.Attempts > bi.Attempts {
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	v := vs[best]
	return &Suggestion{Tool: tool, Cmd: v.Cmd, Attempts: v.Attempts, CostMS: v.CostMS, When: v.TS}
}

// state tracks the last whisper per tool for cooldown purposes.
type state map[string]time.Time

func statePath() (string, error) {
	p, err := index.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "whisper.state"), nil
}

// OnCooldown reports whether tool whispered less than Cooldown ago, and
// stamps it when it did not (so a yes answer means: speak now).
func OnCooldown(tool string, now time.Time) bool {
	p, err := statePath()
	if err != nil {
		return true // when in doubt, stay quiet
	}
	st := state{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if last, ok := st[tool]; ok && now.Sub(last) < Cooldown {
		return true
	}
	st[tool] = now
	if b, err := json.Marshal(st); err == nil {
		_ = os.WriteFile(p, b, 0o600)
	}
	return false
}
