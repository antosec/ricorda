package analyze

import (
	"testing"

	"github.com/antosec/ricorda/internal/history"
)

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

func TestChainsDoNotCrossSources(t *testing.T) {
	mixed := append(entries("bash", "ffmpeg -i a.mp4 out.gif"), entries("zsh", "ffmpeg -i a.mp4 out.gf")...)
	reports := Analyze(mixed)
	r := find(t, reports, "ffmpeg")
	if len(r.HardWon) != 0 {
		t.Fatalf("chain crossed source boundary: %+v", r.HardWon)
	}
}
