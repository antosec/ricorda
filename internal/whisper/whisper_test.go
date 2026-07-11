package whisper

import (
	"testing"
	"time"

	"github.com/antosec/ricorda/internal/index"
)

func idx(tool string, vs ...index.Victory) index.File {
	return index.File{Version: 1, Tools: map[string][]index.Victory{tool: vs}}
}

func TestPickPrefersSameProject(t *testing.T) {
	i := idx("docker",
		index.Victory{Cmd: "docker compose up -d --build", Attempts: 5, CWD: "/other/proj"},
		index.Victory{Cmd: "docker run --rm -v .:/app img", Attempts: 2, CWD: "/work/api"},
	)
	s := Pick(i, "docker", "docker run img", "/work/api")
	if s == nil || s.Cmd != "docker run --rm -v .:/app img" {
		t.Fatalf("same-project victory not preferred: %+v", s)
	}
}

func TestPickFallsBackToHardestFight(t *testing.T) {
	i := idx("git",
		index.Victory{Cmd: "git rebase --onto main feat", Attempts: 4, CWD: "/a"},
		index.Victory{Cmd: "git push --force-with-lease", Attempts: 6, CWD: "/b"},
	)
	s := Pick(i, "git", "git push", "/elsewhere")
	if s == nil || s.Attempts != 6 {
		t.Fatalf("hardest fight not chosen: %+v", s)
	}
}

func TestPickNeverSuggestsTheFailedCommandItself(t *testing.T) {
	i := idx("npm", index.Victory{Cmd: "npm run build", Attempts: 3})
	if s := Pick(i, "npm", "npm run build", ""); s != nil {
		t.Fatalf("suggested the command that just failed: %+v", s)
	}
}

func TestPickSilentWhenUnknown(t *testing.T) {
	i := idx("go", index.Victory{Cmd: "go build ./...", Attempts: 2})
	if s := Pick(i, "cargo", "cargo build", ""); s != nil {
		t.Fatalf("spoke without knowledge: %+v", s)
	}
	if s := Pick(i, "", "whatever", ""); s != nil {
		t.Fatalf("spoke for empty tool: %+v", s)
	}
}

func TestCooldownStampsAndBlocks(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	if OnCooldown("docker", now) {
		t.Fatal("fresh tool should not be on cooldown")
	}
	if !OnCooldown("docker", now.Add(10*time.Minute)) {
		t.Fatal("second whisper within the hour not blocked")
	}
	if OnCooldown("git", now.Add(11*time.Minute)) {
		t.Fatal("cooldown leaked across tools")
	}
	if OnCooldown("docker", now.Add(2*time.Hour)) {
		t.Fatal("cooldown never expired")
	}
}
