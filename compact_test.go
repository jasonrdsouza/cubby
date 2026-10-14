package main

import (
	"bytes"
	"crypto/rand"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestCompactDB(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cubby.db")
	dst := filepath.Join(dir, "cubby.compact.db")

	cubby, err := NewCubbyServer(src, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := cubby.AddAdmin(testAdmin, testPassword); err != nil {
		t.Fatal(err)
	}
	addTestReaderKey(t, cubby, testReader)
	big := make([]byte, 8<<20)
	rand.Read(big)
	err = cubby.db.Update(func(tx *bolt.Tx) error {
		if err := cubby.Put("big", big, tx); err != nil {
			return err
		}
		if err := cubby.Put("keep", []byte("kept"), tx); err != nil {
			return err
		}
		return cubby.PutMetadata("keep", &CubbyMetadata{ContentType: "text/plain", ReaderKeys: []string{testReader}}, tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	// the freed pages stay in the file until it is compacted
	if err := cubby.db.Update(func(tx *bolt.Tx) error { return cubby.Delete("big", tx) }); err != nil {
		t.Fatal(err)
	}

	// the server holds the lock, so compacting a live database must fail
	if _, _, err := CompactDB(src, dst); err == nil {
		t.Fatal("compacted a database the server has open")
	}
	cubby.Close()

	before, after, err := CompactDB(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if after >= before/4 {
		t.Errorf("compacted %d -> %d bytes, want it to reclaim the deleted blob", before, after)
	}
	if _, _, err := CompactDB(src, dst); err == nil {
		t.Error("overwrote an existing destination")
	}

	compacted, err := NewCubbyServer(dst, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer compacted.Close()
	if user := compacted.FetchUser(testAdmin, testPassword); !user.IsAdmin() {
		t.Error("admin missing after compaction")
	}
	compacted.db.View(func(tx *bolt.Tx) error {
		if got := compacted.Get("keep", tx); !bytes.Equal(got, []byte("kept")) {
			t.Errorf("keep = %q after compaction", got)
		}
		if m := compacted.GetMetadata("keep", tx); m.ContentType != "text/plain" || !compacted.FetchUser(testReader, testPassword).CanRead(m) {
			t.Errorf("keep metadata = %s after compaction", m)
		}
		if compacted.Exists("big", tx) {
			t.Error("deleted blob reappeared after compaction")
		}
		return nil
	})
}

func TestFormatBytes(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 999: "999 B", 32768: "32.8 kB", 46809088: "46.8 MB", 3 << 40: "3.3 TB"} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
