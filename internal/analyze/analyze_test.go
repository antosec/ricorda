package analyze

import (
	"testing"
	"time"

	"github.com/antosec/ricorda/internal/history"
	"github.com/antosec/ricorda/internal/journal"
)

var t0 = time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

func jent(offset time.Duration, cmd string, exit int, durMS int64) journal.Entry {
	return journal.Entry{TS: t0.Add(offset), Cmd: cmd, Exit: exit, DurMS: durMS, Shell: "bash", CWD: "/proj"}
}

func entries(src string, cmds ...string) []history.Entry {
	out := make([]history.Entry, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, history.Entry{Command: c, Source: src})
	}
	return out
}

func find(t *testing.T, reports []ToolReport, tool string) ToolReport {
	t.Helper()
	for _, r := range reports {
		if r.Tool == tool {
			return r
		}
	}
	t.Fatalf("no report for %q in %+v", tool, reports)
	return ToolReport{}
}

func TestRetryChainBecomesHardWon(t *testing.T) {
	reports := Analyze(entries("bash",
		"docker run -it ubuntu bahs",
		"docker run -it ubuntu bash",
		"docker run -it --rm ubuntu bash",
	))
	r := find(t, reports, "docker")
	if len(r.HardWon) != 1 {
		t.Fatalf("want exactly 1 hard-won command, got %+v", r.HardWon)
	}
	hw := r.HardWon[0]
	if hw.Command != "docker run -it --rm ubuntu bash" || hw.Attempts != 3 {
		t.Fatalf("unexpected hard-won: %+v", hw)
	}
}

func TestFrequentCommandBecomesHit(t *testing.T) {
	reports := Analyze(entries("zsh",
		"git status", "ls", "git status", "pwd", "git status",
	))
	r := find(t, reports, "git")
	if len(r.Hits) != 1 || r.Hits[0].Count != 3 || r.Hits[0].Command != "git status" {
		t.Fatalf("unexpected hits: %+v", r.Hits)
	}
	if len(r.HardWon) != 0 {
		t.Fatalf("identical repeats are not retries: %+v", r.HardWon)
	}
}

func TestHelpLookupsAreCounted(t *testing.T) {
	reports := Analyze(entries("bash",
		"ffmpeg --help",
		"man ffmpeg",
		"ffmpeg -i in.mp4 out.gif",
	))
	r := find(t, reports, "ffmpeg")
	if r.HelpLookups != 2 {
		t.Fatalf("want 2 help lookups, got %d", r.HelpLookups)
	}
	if r.Total != 1 {
		t.Fatalf("help lookups must not count as usage, got total %d", r.Total)
	}
}

func TestDashHIsNotHelp(t *testing.T) {
	reports := Analyze(entries("bash", "du -h /var", "du -h /var", "du -h /var"))
	r := find(t, reports, "du")
	if r.HelpLookups != 0 {
		t.Fatalf("du -h misread as help lookup: %+v", r)
	}
	if len(r.Hits) != 1 {
		t.Fatalf("want du -h /var as a hit, got %+v", r.Hits)
	}
}

func TestNoiseAndWrappersAreHandled(t *testing.T) {
	reports := Analyze(entries("bash",
		"cd /tmp",
		"sudo systemctl restart nginx",
		"sudo systemctl restart nginx",
		"sudo systemctl restart nginx",
		"FOO=bar npm run dev",
		"FOO=bar npm run dev",
		"FOO=bar npm run dev",
	))
	for _, r := range reports {
		if r.Tool == "cd" || r.Tool == "sudo" {
			t.Fatalf("wrapper/noise leaked as a tool: %+v", r)
		}
	}
	if r := find(t, reports, "systemctl"); r.Total != 3 {
		t.Fatalf("sudo prefix not stripped: %+v", r)
	}
	if r := find(t, reports, "npm"); r.Total != 3 {
		t.Fatalf("env prefix not stripped: %+v", r)
	}
}

func TestDifferentSubcommandsAreNotRetries(t *testing.T) {
	reports := Analyze(entries("bash",
		"docker compose up -d",
		"docker compose logs -f api",
	))
	r := find(t, reports, "docker")
	if len(r.HardWon) != 0 {
		t.Fatalf("distinct subcommands misread as a retry chain: %+v", r.HardWon)
	}
}

func TestCertifiedFightFromJournal(t *testing.T) {
	reports := Certify(nil, []journal.Entry{
		jent(0, "docker run -it ubuntu bahs", 125, 500),
		jent(time.Minute, "docker run -it ubuntu bash", 0, 800),
	})
	r := find(t, reports, "docker")
	if len(r.HardWon) != 1 {
		t.Fatalf("want 1 certified hard-won, got %+v", r.HardWon)
	}
	hw := r.HardWon[0]
	if !hw.Certified || hw.Attempts != 2 {
		t.Fatalf("not certified correctly: %+v", hw)
	}
	if hw.CostMS != time.Minute.Milliseconds()+800 {
		t.Fatalf("wrong cost: %d", hw.CostMS)
	}
	if hw.CWD != "/proj" {
		t.Fatalf("cwd lost: %+v", hw)
	}
	if r.Total != 2 {
		t.Fatalf("journal-only tool total wrong: %d", r.Total)
	}
}

func TestIdenticalFailuresCertify(t *testing.T) {
	// Heuristics cannot see identical retries; ground truth can.
	reports := Certify(nil, []journal.Entry{
		jent(0, "npm run build", 1, 9000),
		jent(time.Minute, "npm run build", 1, 9100),
		jent(2*time.Minute, "npm run build", 0, 8800),
	})
	r := find(t, reports, "npm")
	if len(r.HardWon) != 1 || !r.HardWon[0].Certified || r.HardWon[0].Attempts != 3 {
		t.Fatalf("identical-failure fight missed: %+v", r.HardWon)
	}
}

func TestDifferentIntentSuccessDoesNotClose(t *testing.T) {
	reports := Certify(nil, []journal.Entry{
		jent(0, "docker compose up -d", 1, 300),
		jent(time.Minute, "docker ps", 0, 100),
	})
	for _, r := range reports {
		if len(r.HardWon) != 0 {
			t.Fatalf("unrelated success closed a fight: %+v", r)
		}
	}
}

func TestLongGapAbandonsFight(t *testing.T) {
	reports := Certify(nil, []journal.Entry{
		jent(0, "cargo build --release", 101, 4000),
		jent(20*time.Minute, "cargo build --release", 0, 3900),
	})
	for _, r := range reports {
		if len(r.HardWon) != 0 {
			t.Fatalf("20-minute gap still counted as one fight: %+v", r)
		}
	}
}

func TestCertifyUpgradesHeuristicEntry(t *testing.T) {
	reports := Analyze(entries("bash",
		"docker run -it ubuntu bahs",
		"docker run -it ubuntu bash",
		"docker run -it --rm ubuntu bash",
	))
	reports = Certify(reports, []journal.Entry{
		jent(0, "docker run -it ubuntu bash", 125, 200),
		jent(time.Minute, "docker run -it --rm ubuntu bash", 0, 700),
	})
	r := find(t, reports, "docker")
	if len(r.HardWon) != 1 {
		t.Fatalf("upgrade duplicated the entry: %+v", r.HardWon)
	}
	hw := r.HardWon[0]
	if !hw.Certified || hw.Attempts != 3 || hw.CostMS == 0 {
		t.Fatalf("upgrade lost data: %+v", hw)
	}
}

func TestChainsDoNotCrossSources(t *testing.T) {
	mixed := append(entries("bash", "ffmpeg -i a.mp4 out.gif"), entries("zsh", "ffmpeg -i a.mp4 out.gf")...)
	reports := Analyze(mixed)
	r := find(t, reports, "ffmpeg")
	if len(r.HardWon) != 0 {
		t.Fatalf("chain crossed source boundary: %+v", r.HardWon)
	}
}
