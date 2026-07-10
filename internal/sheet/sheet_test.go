package sheet

import (
	"os"
	"strings"
	"testing"

	"github.com/antosec/ricorda/internal/analyze"
)

func TestWritePreservesUserNotes(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	r := analyze.ToolReport{
		Tool:    "docker",
		HardWon: []analyze.HardWon{{Command: "docker ps -a", Attempts: 2}},
	}

	path, err := Write(r, "2026-07-10")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate the user adding a note below the keep-marker.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, "\nMY PRECIOUS NOTE\n"...), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Write(r, "2026-07-11"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(after)

	if !strings.Contains(s, "MY PRECIOUS NOTE") {
		t.Fatal("user note was lost on rescan")
	}
	if !strings.Contains(s, "2026-07-11") || strings.Contains(s, "2026-07-10") {
		t.Fatal("generated part was not rewritten")
	}
	if strings.Count(s, KeepMarker) != 1 {
		t.Fatal("keep-marker duplicated")
	}
}

func TestPathForRejectsUnsafeNames(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	for _, bad := range []string{"", "..", "a/b", `a\b`, "CON:", "-flag", "#"} {
		if _, err := PathFor(bad); err == nil {
			t.Errorf("PathFor(%q) accepted an unsafe name", bad)
		}
	}
	if _, err := PathFor("g++"); err != nil {
		t.Errorf("PathFor(g++) should be valid: %v", err)
	}
}

func TestListEmptyWhenNothingScanned(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	metas, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 0 {
		t.Fatalf("want empty list, got %+v", metas)
	}
}
