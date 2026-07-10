package history

import (
	"strings"
	"testing"
)

func cmds(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Command
	}
	return out
}

func joined(entries []Entry) string {
	return strings.Join(cmds(entries), "|")
}

func TestParseBashSkipsTimestampLines(t *testing.T) {
	in := "#1720000000\ngit push\n\nls -la\n# a real comment I typed\n"
	got, err := ParseBash(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "git push|ls -la|# a real comment I typed"
	if joined(got) != want {
		t.Fatalf("got %q, want %q", joined(got), want)
	}
}

func TestParseZshExtendedFormat(t *testing.T) {
	in := ": 1720000000:0;git status\nplain command\n: 1720000001:2;docker ps -a\n"
	got, err := ParseZsh(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "git status|plain command|docker ps -a"
	if joined(got) != want {
		t.Fatalf("got %q, want %q", joined(got), want)
	}
}

func TestParseFishExtractsCmdLines(t *testing.T) {
	in := "- cmd: git status\n  when: 1720000000\n- cmd: npm run dev\n  when: 1720000001\n"
	got, err := ParseFish(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "git status|npm run dev"
	if joined(got) != want {
		t.Fatalf("got %q, want %q", joined(got), want)
	}
}

func TestParsePwshJoinsMultilineCommands(t *testing.T) {
	in := "docker run `\n  -it ubuntu bash\ngit status\ntrailing `"
	got, err := ParsePwsh(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "docker run -it ubuntu bash|git status|trailing"
	if joined(got) != want {
		t.Fatalf("got %q, want %q", joined(got), want)
	}
}
