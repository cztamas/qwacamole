package qwacamole

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInFlightOperationRetainsOwnershipPastModuleCleanup(t *testing.T) {
	dir := t.TempDir()
	lock, e := os.OpenFile(filepath.Join(dir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := BindingConfig{}
	cfg.Dir = dir
	s := &session{refs: 1}
	o := &owner{cfg: cfg, refs: 1, lock: lock, sessions: map[context.Context]*session{ctx: s}}
	registry.Lock()
	registry.entries[dir] = o
	registry.Unlock()
	l := &Lease{owner: o, session: s, ctx: ctx}
	release, e := l.borrow()
	if e != nil {
		t.Fatal(e)
	}
	l.Release()
	cancel()
	registry.Lock()
	kept := registry.entries[dir] == o && o.refs == 1
	registry.Unlock()
	if !kept {
		t.Fatal("in-flight operation lost state ownership")
	}
	if _, e = l.borrow(); e == nil {
		t.Fatal("unloaded configuration started a new operation")
	}
	release()
	release()
	l.Release()
	registry.Lock()
	_, exists := registry.entries[dir]
	registry.Unlock()
	if exists {
		t.Fatal("entry leaked after last operation")
	}
	if _, e = lock.Stat(); e == nil {
		t.Fatal("state lock still open")
	}
}
