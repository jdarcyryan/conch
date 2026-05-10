package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	c, err := New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestGetFreshDownload(t *testing.T) {
	c := newTestCache(t)

	payload := []byte("hello world")
	want := sha256Hex(payload)

	calls := 0
	fetch := func(w io.Writer) error {
		calls++
		_, err := w.Write(payload)
		return err
	}

	path, err := c.Get("pwsh", "x.bin", want, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected 1 fetch, got %d", calls)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("file contents mismatch")
	}

	// Second call should hit the cache (no fetch).
	path2, err := c.Get("pwsh", "x.bin", want, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected still 1 fetch, got %d", calls)
	}
	if path != path2 {
		t.Errorf("path mismatch on 2nd call")
	}
}

func TestGetReplacesCorruptedFile(t *testing.T) {
	c := newTestCache(t)

	payload := []byte("good content")
	want := sha256Hex(payload)

	// Pre-seed with garbage.
	dest, _ := c.Path("pwsh", "y.bin")
	if err := os.WriteFile(dest, []byte("CORRUPTED"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := c.Get("pwsh", "y.bin", want, func(w io.Writer) error {
		_, err := w.Write(payload)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(payload) {
		t.Errorf("did not replace corrupted file: got %q", got)
	}
}

func TestGetRejectsHashMismatch(t *testing.T) {
	c := newTestCache(t)

	_, err := c.Get("pwsh", "z.bin", sha256Hex([]byte("expected")), func(w io.Writer) error {
		_, err := w.Write([]byte("actual"))
		return err
	})
	if err == nil {
		t.Fatal("expected hash mismatch error, got nil")
	}

	dest, _ := c.Path("pwsh", "z.bin")
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("rejected file should not be left on disk: stat err = %v", err)
	}
}

func TestGetEmptyExpectedSHARefetchesEveryCall(t *testing.T) {
	// Empty expected hash means the caller has no trust anchor for
	// the cached file. The cache must refuse to short-circuit on a
	// pre-existing file in that mode, otherwise a tampered cache
	// would silently win on subsequent installs. Verifying bytes is
	// the fetcher's responsibility in this regime.
	c := newTestCache(t)

	calls := 0
	fetch := func(w io.Writer) error {
		calls++
		_, err := w.Write([]byte("anything"))
		return err
	}

	if _, err := c.Get("pwsh", "trust.bin", "", fetch); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get("pwsh", "trust.bin", "", fetch); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected 2 fetches (empty hash distrusts cache), got %d", calls)
	}
}
