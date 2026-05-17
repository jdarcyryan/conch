//go:build !windows

package pwsh

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestExtractTarGzPreservesExecuteBit guards against the regression
// where an extracted pwsh came out without owner-execute under a
// restrictive umask, producing "fork/exec: permission denied" at run
// time. Umask is forced to 0o077 so any reliance on the host's umask
// would strip the execute bit and fail the assertion.
func TestExtractTarGzPreservesExecuteBit(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	body := []byte("#!/bin/sh\necho hi\n")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "pwsh",
		Mode:     0o755,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}

	dir := t.TempDir()
	archive := filepath.Join(dir, "pwsh.tar.gz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	info, err := os.Stat(filepath.Join(dest, "pwsh"))
	if err != nil {
		t.Fatalf("stat extracted pwsh: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("extracted pwsh is not owner-executable: mode = %v", info.Mode().Perm())
	}
}
