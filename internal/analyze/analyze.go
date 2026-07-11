// Package analyze turns raw history entries into per-tool reports of the
// commands worth remembering.
package analyze

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/antosec/ricorda/internal/history"
	"github.com/antosec/ricorda/internal/journal"
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
// retries — the one you fought for. Certified entries come from the journal
// (real exit codes); the rest are inferred from history files.
type HardWon struct {
	Command   string
	Attempts  int
	Certified bool
	CostMS    int64 // wall time from first failure to victory (certified only)
	CWD       string
	TS        time.Time
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
	// consecutive attempts at the same thing. High enough that different
	// subcommands sharing a long prefix (docker compose up / logs) don't
	// read as retries.
	simLow = 0.65
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

// maxFightGap is the longest pause between attempts that still counts as
// the same fight; beyond it you probably walked away.
const maxFightGap = 15 * time.Minute

// Certify overlays ground truth from the journal onto heuristic reports:
// fail-runs closed by a success become certified hard-won commands with a
// real cost, upgrading a matching heuristic entry or being added. Tools
// seen only in the journal gain their own report.
func Certify(reports []ToolReport, jour []journal.Entry) []ToolReport {
	fights, journalTotals := certifiedFights(jour)
	if len(fights) == 0 {
		return reports
	}

	idx := make(map[string]int, len(reports))
	for i, r := range reports {
		idx[r.Tool] = i
	}

	for _, f := range fights {
		i, ok := idx[f.tool]
		if !ok {
			reports = append(reports, ToolReport{Tool: f.tool, Total: journalTotals[f.tool]})
			i = len(reports) - 1
			idx[f.tool] = i
		}
		r := &reports[i]

		upgraded := false
		for j := range r.HardWon {
			if r.HardWon[j].Command == f.hw.Command {
				if f.hw.Attempts > r.HardWon[j].Attempts {
					r.HardWon[j].Attempts = f.hw.Attempts
				}
				r.HardWon[j].Certified = true
				r.HardWon[j].CostMS += f.hw.CostMS
				r.HardWon[j].CWD = f.hw.CWD
				r.HardWon[j].TS = f.hw.TS
				upgraded = true
				break
			}
		}
		if !upgraded {
			r.HardWon = append(r.HardWon, f.hw)
		}

		sort.Slice(r.HardWon, func(a, b int) bool {
			if r.HardWon[a].Certified != r.HardWon[b].Certified {
				return r.HardWon[a].Certified
			}
			if r.HardWon[a].Attempts != r.HardWon[b].Attempts {
				return r.HardWon[a].Attempts > r.HardWon[b].Attempts
			}
			return r.HardWon[a].Command < r.HardWon[b].Command
		})
		if len(r.HardWon) > maxHardWon {
			r.HardWon = r.HardWon[:maxHardWon]
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

type fight struct {
	tool string
	hw   HardWon
}

// certifiedFights walks the journal per tool and shell, collecting fail-runs
// closed by a same-intent success. It also returns per-tool journal counts
// for tools that never made it into a history file.
func certifiedFights(jour []journal.Entry) ([]fight, map[string]int) {
	type seqKey struct{ tool, shell string }
	seqs := map[seqKey][]journal.Entry{}
	var order []seqKey
	totals := map[string]int{}

	for _, e := range jour {
		// Journal entries are redacted at write time; cleaning again here
		// costs little and protects hand-edited or imported journals.
		cmd := redact.Clean(normalize(e.Cmd))
		if cmd == "" {
			continue
		}
		tool, _, isHelp := classify(cmd)
		if isHelp || tool == "" || noise[tool] {
			continue
		}
		totals[tool]++
		k := seqKey{tool, e.Shell}
		if _, seen := seqs[k]; !seen {
			order = append(order, k)
		}
		e.Cmd = cmd
		seqs[k] = append(seqs[k], e)
	}

	var out []fight
	for _, k := range order {
		var run []journal.Entry
		for _, e := range seqs[k] {
			if len(run) > 0 {
				prev := run[len(run)-1]
				if !sameIntent(prev.Cmd, e.Cmd) || e.TS.Sub(prev.TS) > maxFightGap {
					run = nil
				}
			}
			if e.Exit != 0 {
				run = append(run, e)
				continue
			}
			if len(run) > 0 {
				cost := e.TS.Sub(run[0].TS).Milliseconds() + e.DurMS
				if cost < 0 {
					cost = 0
				}
				out = append(out, fight{tool: k.tool, hw: HardWon{
					Command:   e.Cmd,
					Attempts:  len(run) + 1,
					Certified: true,
					CostMS:    cost,
					CWD:       e.CWD,
					TS:        e.TS,
				}})
				run = nil
			}
		}
	}
	return out, totals
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
// same thing: same intent at token level, close in edit distance, and not
// identical.
func similar(a, b string) bool {
	if a == b {
		return false
	}
	if !sameIntent(a, b) {
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

// sameIntent guards against long shared prefixes making different
// subcommands look like retries (docker compose up vs docker compose logs).
// At the first place the token streams diverge, either one side inserted
// tokens (a flag added on retry) or the diverging tokens themselves must
// look like small edits of each other.
func sameIntent(a, b string) bool {
	ta, tb := strings.Fields(a), strings.Fields(b)
	n := min(len(ta), len(tb))
	for i := 0; i < n; i++ {
		if ta[i] == tb[i] {
			continue
		}
		if containsNear(tb, i+1, ta[i]) || containsNear(ta, i+1, tb[i]) {
			return true
		}
		return tokenSimilar(ta[i], tb[i])
	}
	return true // one command is a token-prefix of the other
}

// containsNear reports whether tok appears within the next few tokens,
// which is the signature of an insertion rather than a replacement.
func containsNear(ts []string, from int, tok string) bool {
	for i := from; i < len(ts) && i < from+3; i++ {
		if ts[i] == tok {
			return true
		}
	}
	return false
}

// tokenSimilar reports whether two diverging tokens look like a typo fix or
// a value extension of one another.
func tokenSimilar(a, b string) bool {
	a, b = strings.Trim(a, `"'`), strings.Trim(b, `"'`)
	if len(a) >= 3 && len(b) >= 3 && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a)) {
		return true
	}
	longest := max(len(a), len(b))
	if longest == 0 {
		return false
	}
	return 1-float64(levenshtein(a, b))/float64(longest) >= 0.5
}

func clip(s string) string {
	if len(s) > simMaxLen {
		return s[:simMaxLen]
	}
	return s
}

// ToolOf extracts the tool a command belongs to, or "" when the command is
// noise or a help lookup. It is the single classification entry point for
// callers outside analysis (the whisper path).
func ToolOf(cmd string) string {
	c := normalize(cmd)
	if c == "" {
		return ""
	}
	tool, _, isHelp := classify(c)
	if isHelp || tool == "" || noise[tool] {
		return ""
	}
	return tool
}

// FmtDurMS renders a fight cost for humans: 45s, 23m, 1h05m.
func FmtDurMS(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
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
