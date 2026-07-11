package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpsertIsIdempotent(t *testing.T) {
	block := BlockFor("bash")
	once := Upsert("# my rc file\nalias ll='ls -la'\n", block)
	twice := Upsert(once, block)
	if once != twice {
		t.Fatal("double install changed the file")
	}
	if strings.Count(twice, BeginMark) != 1 || strings.Count(twice, EndMark) != 1 {
		t.Fatalf("expected exactly one block:\n%s", twice)
	}
	if !strings.Contains(twice, "alias ll='ls -la'") {
		t.Fatal("user content lost")
	}
}

func TestUpsertReplacesOldBlockInPlace(t *testing.T) {
	old := "before\n" + BeginMark + "\nOLD CONTENT v0\n" + EndMark + "\nafter\n"
	got := Upsert(old, BlockFor("zsh"))
	if strings.Contains(got, "OLD CONTENT v0") {
		t.Fatal("old block content survived an upgrade")
	}
	if !strings.Contains(got, "before\n") || !strings.Contains(got, "\nafter\n") {
		t.Fatalf("surrounding content damaged:\n%s", got)
	}
}

func TestRemoveIsSurgical(t *testing.T) {
	content := Upsert("top\n", BlockFor("bash")) + "bottom\n"
	out, changed := Remove(content)
	if !changed {
		t.Fatal("nothing removed")
	}
	if strings.Contains(out, BeginMark) || strings.Contains(out, "__ricorda_prompt") {
		t.Fatalf("block residue left:\n%s", out)
	}
	if !strings.Contains(out, "top\n") || !strings.Contains(out, "bottom\n") {
		t.Fatalf("user content damaged:\n%s", out)
	}
	if _, changedAgain := Remove(out); changedAgain {
		t.Fatal("second remove should be a no-op")
	}
}

func TestMalformedBlockIsLeftAlone(t *testing.T) {
	content := "keep\n" + BeginMark + "\nno end marker here\n"
	if _, changed := Remove(content); changed {
		t.Fatal("malformed block must not be touched")
	}
	got := Upsert(content, BlockFor("bash"))
	if !strings.Contains(got, "no end marker here") {
		t.Fatal("malformed content eaten by upsert")
	}
}

func TestInstallAndUninstallFile(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".bashrc")
	if err := os.WriteFile(rc, []byte("export FOO=bar\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := InstallFile(rc, BlockFor("bash"), false); err != nil {
		t.Fatal(err)
	}
	if !Installed(rc) {
		t.Fatal("block not detected after install")
	}
	if runtime.GOOS != "windows" { // POSIX permissions are synthetic on Windows
		info, _ := os.Stat(rc)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file permissions changed: %v", info.Mode().Perm())
		}
	}

	removed, err := UninstallFile(rc, false)
	if err != nil || !removed {
		t.Fatalf("uninstall failed: %v %v", removed, err)
	}
	b, _ := os.ReadFile(rc)
	if !strings.Contains(string(b), "export FOO=bar") {
		t.Fatal("user content lost on uninstall")
	}
	if Installed(rc) {
		t.Fatal("block still detected after uninstall")
	}
}

func TestOwnFileLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conf.d", "ricorda.fish")

	if err := InstallFile(path, BlockFor("fish"), true); err != nil {
		t.Fatal(err)
	}
	if !Installed(path) {
		t.Fatal("own file not installed")
	}
	removed, err := UninstallFile(path, true)
	if err != nil || !removed {
		t.Fatalf("own file uninstall failed: %v %v", removed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("own file still exists")
	}
	if removed, _ := UninstallFile(path, true); removed {
		t.Fatal("second uninstall should be a no-op")
	}
}

func TestEveryShellHasABlock(t *testing.T) {
	for _, s := range Shells() {
		b := BlockFor(s)
		if b == "" {
			t.Fatalf("no block for %s", s)
		}
		if !strings.HasPrefix(b, BeginMark) || !strings.HasSuffix(b, EndMark) {
			t.Fatalf("block for %s not properly delimited", s)
		}
		if !strings.Contains(b, "journal add --shell "+s) {
			t.Fatalf("block for %s does not feed the journal", s)
		}
		if !strings.Contains(b, "ricorda whisper --exit") {
			t.Fatalf("block for %s never whispers on failure", s)
		}
	}
	if BlockFor("nushell") != "" {
		t.Fatal("unknown shell should return empty block")
	}
}
