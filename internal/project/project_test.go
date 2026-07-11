package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeFoldsWSLAndWindows(t *testing.T) {
	cases := map[string]string{
		`C:\Users\Dev\proj`:     "c:/users/dev/proj",
		`c:/users/dev/proj/`:    "c:/users/dev/proj",
		`/mnt/c/Users/Dev/proj`: "c:/users/dev/proj",
		`/home/dev/proj`:        "/home/dev/proj",
		`/mnt/d/data`:           "d:/data",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
	if Normalize(`/mnt/c/x`) != Normalize(`C:\x`) {
		t.Error("WSL and Windows views of the same dir do not match")
	}
}

func TestRootFindsGitAncestor(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repo, "internal", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Root(nested); got != repo {
		t.Fatalf("Root(%q) = %q, want %q", nested, got, repo)
	}
	if !Same(nested, repo) {
		t.Fatal("nested dir not recognized as same project")
	}
}

func TestRootFallsBackToDirItself(t *testing.T) {
	dir := t.TempDir() // no .git anywhere under TempDir ancestry, typically
	got := Root(filepath.Join(dir, "ghost", "missing"))
	if got == "" {
		t.Fatal("Root returned empty for a nonexistent dir")
	}
}

func TestLabelIsLeafOnly(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Base(repo)
	if got := Label(filepath.Join(repo, "sub")); got != want {
		t.Fatalf("Label = %q, want %q", got, want)
	}
	if Label("") != "" {
		t.Fatal("empty dir should have empty label")
	}
}
