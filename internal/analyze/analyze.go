// Package analyze turns raw history entries into per-tool reports of the
// commands worth remembering.
package analyze

import (
	"sort"
	"strings"

	"github.com/antosec/ricorda/internal/history"
	"github.com/antosec/ricorda/internal/redact"
)

// ToolReport summarizes one tool's usage.
type ToolReport struct {
	Tool        string
	Total       int
	HardWon     []HardWon
	Hits        []Hit
	HelpLookups int
}

// HardWon is the final, working form of a command reached after visible
// retries — the one you fought for.
type HardWon struct {
	Command  string
	Attempts int
}

// Hit is a frequently repeated non-trivial command.
type Hit struct {
	Command string
	Count   int
}

const (
	maxHardWon = 8
	maxHits    = 8
	minHits    = 3
	// chainWindow is how many commands apart two attempts may sit and still
	// count as the same fight.
	chainWindow = 3
	// simLow is the minimum edit-similarity for two commands to look like
	// consecutive attempts at the same thing.
	simLow = 0.55
	// simMaxLen caps the length fed to the edit-distance computation.
	simMaxLen = 300
)

// noise are commands too trivial to deserve a cheatsheet.
var noise = map[string]bool{
	"cd": true, "ls": true, "ll": true, "la": true, "dir": true,
	"clear": true, "cls": true, "exit": true, "pwd": true,
	"history": true, "whoami": true, "echo": true, "cat": true, "type": true,
}

type record struct {
	cmd string
	src string
	pos int
}

// Analyze groups entries by tool and extracts hard-won commands, greatest
// hits and help lookups. Every command is redacted before it leaves this
// package.
func Analyze(entries []history.Entry) []ToolReport {
	perTool := map[string][]record{}
	helpFor := map[string]int{}

	for i, e := range entries {
		cmd := normalize(e.Command)
		if cmd == "" {
			continue
		}
		tool, helpTarget, isHelp := classify(cmd)
		if isHelp {
			if helpTarget != "" && !noise[helpTarget] {
				helpFor[helpTarget]++
			}
			continue
		}
		if tool == "" || noise[tool] {
			continue
		}
		perTool[tool] = append(perTool[tool], record{cmd: cmd, src: e.Source, pos: i})
	}

	var reports []ToolReport
	for tool, recs := range perTool {
		r := ToolReport{Tool: tool, Total: len(recs), HelpLookups: helpFor[tool]}
		r.Hits = greatestHits(recs)
		r.HardWon = hardWon(recs)
		reports = append(reports, r)
	}

	// Tools the user only ever asked help about still deserve a mention.
	for tool, n := range helpFor {
		if _, ok := perTool[tool]; !ok {
			reports = append(reports, ToolReport{Tool: tool, HelpLookups: n})
		}
	}

	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Total != reports[j].Total {
			return reports[i].Total > reports[j].Total
		}
		return reports[i].Tool < reports[j].Tool
	})
	return reports
}

// greatestHits returns the most-repeated non-trivial commands.
func greatestHits(recs []record) []Hit {
	counts := map[string]int{}
	for _, rec := range recs {
		counts[rec.cmd]++
	}
	var hits []Hit
	for cmd, n := range counts {
		if n >= minHits && len(strings.Fields(cmd)) >= 2 {
			hits = append(hits, Hit{Command: redact.Clean(cmd), Count: n})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Count != hits[j].Count {
			return hits[i].Count > hits[j].Count
		}
		return hits[i].Command < hits[j].Command
	})
	if len(hits) > maxHits {
		hits = hits[:maxHits]
	}
	return hits
}

// hardWon finds chains of near-identical consecutive attempts and keeps the
// last command of each chain — the one that (presumably) finally worked.
func hardWon(recs []record) []HardWon {
	best := map[string]int{} // final command -> attempts
	endChain := func(final string, length int) {
		if length < 2 {
			return
		}
		final = redact.Clean(final)
		if length > best[final] {
			best[final] = length
		}
	}

	chainLen := 1
	for i := 1; i < len(recs); i++ {
		prev, cur := recs[i-1], recs[i]
		if cur.src == prev.src && cur.pos-prev.pos <= chainWindow && similar(prev.cmd, cur.cmd) {
			chainLen++
			continue
		}
		endChain(prev.cmd, chainLen)
		chainLen = 1
	}
	if len(recs) > 0 {
		endChain(recs[len(recs)-1].cmd, chainLen)
	}

	var out []HardWon
	for cmd, n := range best {
		out = append(out, HardWon{Command: cmd, Attempts: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Attempts != out[j].Attempts {
			return out[i].Attempts > out[j].Attempts
		}
		return out[i].Command < out[j].Command
	})
	if len(out) > maxHardWon {
		out = out[:maxHardWon]
	}
	return out
}

// classify extracts the tool a command belongs to and whether the command is
// a help lookup (man/tldr/help, or a --help style flag).
func classify(cmd string) (tool, helpTarget string, isHelp bool) {
	fields := strings.Fields(cmd)
	for len(fields) > 0 {
		f := fields[0]
		if strings.Contains(f, "=") && !strings.HasPrefix(f, "=") {
			fields = fields[1:] // FOO=bar prefix
			continue
		}
		if f == "sudo" || f == "doas" || f == "time" || f == "nohup" {
			fields = fields[1:]
			continue
		}
		break
	}
	if len(fields) == 0 {
		return "", "", false
	}
	tool = baseName(strings.ToLower(fields[0]))

	switch tool {
	case "man", "tldr", "help":
		if len(fields) > 1 {
			return tool, baseName(strings.ToLower(fields[len(fields)-1])), true
		}
		return tool, "", false
	}
	for _, f := range fields[1:] {
		// A bare -h is too ambiguous (du -h, df -h) to count as help.
		if f == "--help" || f == "-?" || f == "/?" {
			return tool, tool, true
		}
	}
	return tool, "", false
}

// baseName strips directories and common Windows suffixes so /usr/bin/git,
// git.exe and .\build.ps1 all group cleanly.
func baseName(s string) string {
	s = strings.Trim(s, `"'`)
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1"} {
		s = strings.TrimSuffix(s, ext)
	}
	return s
}

// normalize collapses whitespace so cosmetic differences don't split
// otherwise identical commands.
func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// similar reports whether two commands look like consecutive attempts at the
// same thing: close in edit distance but not identical.
func similar(a, b string) bool {
	if a == b {
		return false
	}
	a, b = clip(a), clip(b)
	longest := len(a)
	if len(b) > longest {
		longest = len(b)
	}
	if longest == 0 {
		return false
	}
	d := levenshtein(a, b)
	return 1-float64(d)/float64(longest) >= simLow
}

func clip(s string) string {
	if len(s) > simMaxLen {
		return s[:simMaxLen]
	}
	return s
}

// levenshtein computes edit distance with the classic two-row method.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			m := prev[j] + 1
			if cur[j-1]+1 < m {
				m = cur[j-1] + 1
			}
			if prev[j-1]+cost < m {
				m = prev[j-1] + cost
			}
			cur[j] = m
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
