package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicReplacementAndFailedCommitLeavesTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "current.json")
	if e := Atomic(path, []byte("first")); e != nil {
		t.Fatal(e)
	}
	if e := Atomic(path, []byte("second")); e != nil {
		t.Fatal(e)
	}
	d, e := os.ReadFile(path)
	if e != nil || string(d) != "second" {
		t.Fatal("replacement not durable", e)
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unexpected manifest permissions", e)
	}
	if e := Atomic(dir, []byte("invalid target")); e == nil {
		t.Fatal("directory overwritten")
	}
	names, e := os.ReadDir(dir)
	if e != nil || len(names) != 1 {
		t.Fatal("temporary candidate leaked", e)
	}
}
