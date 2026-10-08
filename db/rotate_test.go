package db

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestRotateReturnsWhatItFroze(t *testing.T) {
	d := mustOpen(t, t.TempDir())
	_ = d.Put([]byte("a"), []byte("1"))
	oldMem, oldWAL := d.mem, d.wal

	fm, err := d.rotate(100)
	if err != nil {
		t.Fatal(err)
	}
	if fm.mem != oldMem || fm.walPath != oldWAL.GetWriterPath() {
		t.Fatal("rotate returned something other than the memtable and log it froze")
	}
	if d.imm[len(d.imm)-1] != fm {
		t.Fatal("the returned entry must be the one appended to imm")
	}
	if d.mem == oldMem || d.wal == oldWAL {
		t.Fatal("active memtable and log should have been replaced")
	}
	if err := oldWAL.Append([]byte("x")); err == nil {
		t.Fatal("the old log writer should have been closed")
	}
}

func TestRotateFailureChangesNothing(t *testing.T) {
	d := mustOpen(t, t.TempDir())
	_ = d.Put([]byte("a"), []byte("1"))
	mem, w, n := d.mem, d.wal, len(d.imm)

	d.dir = filepath.Join(d.dir, "does-not-exist") // the new log cannot be created
	if _, err := d.rotate(100); err == nil {
		t.Fatal("rotate should fail when the log cannot be created")
	}
	if d.mem != mem || d.wal != w || len(d.imm) != n {
		t.Fatal("a failed rotation must change nothing")
	}
}

func TestCloseDoesNotHangAfterRotations(t *testing.T) {
	d := mustOpen(t, t.TempDir())
	d.memLimit = 1 // every write rotates and sends to the flusher
	for i := 0; i < 8; i++ {
		if err := d.Put([]byte(fmt.Sprintf("k%d", i)), []byte("v")); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan struct{})
	go func() { d.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung: the flusher or the write worker did not exit")
	}
}
