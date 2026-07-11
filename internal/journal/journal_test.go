package journal

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAndReadAllChronological(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())

	june := Entry{TS: time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC), Cmd: "git push", Exit: 0, DurMS: 900, Shell: "bash"}
	july := Entry{TS: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Cmd: "docker ps", Exit: 1, DurMS: 40, Shell: "pwsh"}

	// Write out of order: reading must still be chronological.
	if err := Append(july); err != nil {
		t.Fatal(err)
	}
	if err := Append(june); err != nil {
		t.Fatal(err)
	}

	d, _ := Dir()
	for _, want := range []string{"2026-06.jsonl", "2026-07.jsonl"} {
		if _, err := os.Stat(filepath.Join(d, want)); err != nil {
			t.Fatalf("expected monthly file %s: %v", want, err)
		}
	}

	got, err := ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Cmd != "git push" || got[1].Cmd != "docker ps" {
		t.Fatalf("unexpected read order: %+v", got)
	}
	if got[1].Exit != 1 || got[1].DurMS != 40 || got[1].Shell != "pwsh" {
		t.Fatalf("fields lost in roundtrip: %+v", got[1])
	}
}

func TestAppendRedactsBeforeDisk(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	e := Entry{TS: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Cmd: "export API_TOKEN=supersecret123", Exit: 0}
	if err := Append(e); err != nil {
		t.Fatal(err)
	}
	d, _ := Dir()
	raw, err := os.ReadFile(filepath.Join(d, "2026-07.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "supersecret123") {
		t.Fatal("secret reached disk unredacted")
	}
}

func TestAppendDropsEmptyCommands(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	if err := Append(Entry{Cmd: "   "}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty command was stored: %+v", got)
	}
}

func TestReadAllSkipsCorruptLines(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	if err := Append(Entry{TS: time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC), Cmd: "ls -la", Exit: 0}); err != nil {
		t.Fatal(err)
	}
	d, _ := Dir()
	path := filepath.Join(d, "2026-07.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not json}\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Cmd != "ls -la" {
		t.Fatalf("corrupt line broke reading: %+v", got)
	}
}

func TestDecodeB64(t *testing.T) {
	in := `docker run -v "C:\data":/data img`
	enc := base64.StdEncoding.EncodeToString([]byte(in))
	got, err := DecodeB64(enc + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("roundtrip failed: %q", got)
	}
	if _, err := DecodeB64("not_base64!!!"); err == nil {
		t.Fatal("invalid base64 accepted")
	}
}
