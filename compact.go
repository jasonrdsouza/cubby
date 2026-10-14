package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

// compactTxMaxSize bounds each transaction while copying, so compacting a
// large database doesn't hold everything in memory at once.
const compactTxMaxSize = 64 << 20

// CompactDB copies the live contents of the database at src into a new file at
// dst. Bolt files never shrink, so space freed by deleted or overwritten blobs
// is only reclaimed this way. src is opened read-only and never modified, and
// dst must not already exist; swapping the files is left to the caller. It
// returns both file sizes in bytes.
func CompactDB(src, dst string) (before, after int64, err error) {
	if _, err := os.Stat(dst); err == nil {
		return 0, 0, fmt.Errorf("%s already exists", dst)
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, 0, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return 0, 0, err
	}

	srcDB, err := bolt.Open(src, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return 0, 0, fmt.Errorf("opening %s (is the server still running?): %w", src, err)
	}
	defer srcDB.Close()

	dstDB, err := bolt.Open(dst, info.Mode().Perm(), &bolt.Options{Timeout: time.Second})
	if err != nil {
		return 0, 0, err
	}
	err = bolt.Compact(dstDB, srcDB, compactTxMaxSize)
	if closeErr := dstDB.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
		return 0, 0, err
	}

	compacted, err := os.Stat(dst)
	if err != nil {
		return 0, 0, err
	}
	return info.Size(), compacted.Size(), nil
}

// formatBytes renders a byte count for humans, eg. "46.8 MB".
func formatBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
