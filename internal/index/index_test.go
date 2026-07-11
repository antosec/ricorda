package index

import (
	"os"
	"testing"
	"time"

	"github.com/antosec/ricorda/internal/analyze"
)

func TestWriteKeepsOnlyCertifiedAndReadsBack(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())
	ts := time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC)

	reports := []analyze.ToolReport{
		{Tool: "docker", HardWon: []analyze.HardWon{
			{Command: "docker run --rm img", Attempts: 4, Certified: true, CostMS: 120000, CWD: "/proj", TS: ts},
			{Command: "docker guessed-only", Attempts: 3, Certified: false},
		}},
		{Tool: "ls-like", HardWon: []analyze.HardWon{{Command: "only heuristic", Attempts: 2}}},
	}
	if err := Write(reports); err != nil {
		t.Fatal(err)
	}

	f, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Tools) != 1 {
		t.Fatalf("uncertified entries leaked into index: %+v", f.Tools)
	}
	vs := f.Tools["docker"]
	if len(vs) != 1 || vs[0].Cmd != "docker run --rm img" || vs[0].CostMS != 120000 || !vs[0].TS.Equal(ts) {
		t.Fatalf("roundtrip lost data: %+v", vs)
	}
}

func TestReadMissingAndCorrupt(t *testing.T) {
	t.Setenv("RICORDA_HOME", t.TempDir())

	f, err := Read()
	if err != nil || len(f.Tools) != 0 {
		t.Fatalf("missing index should read empty: %+v %v", f, err)
	}

	p, _ := Path()
	if err := os.MkdirAll(t.TempDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err = Read()
	if err != nil || f.Tools == nil {
		t.Fatalf("corrupt index must degrade to empty, got %+v %v", f, err)
	}
}
