package lsmdb

import (
	"errors"
	"testing"
)

func TestDBPutGet(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get([]byte("missing")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	_ = db.Put([]byte("name"), []byte("fouad"))
	_ = db.Put([]byte("name"), []byte("fouad2"))
	got, err := db.Get([]byte("name"))
	if err != nil || string(got) != "fouad2" {
		t.Fatalf("got %q, %v", got, err)
	}
}
