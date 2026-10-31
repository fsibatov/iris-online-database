package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Version retention counts version directories, not loose metadata files.
// Otherwise a newer marker file can evict a real, recoverable version.
func TestCleanupVersionsIgnoresLooseFilesWhenCountingRetention(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-7 * 24 * time.Hour)
	for i, name := range []string{"1.0", "2.0", "3.0"} {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "payload.txt"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	for i, name := range []string{"manifest.json", "notes.txt"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("metadata"), 0o600); err != nil {
			t.Fatal(err)
		}
		stamp := base.Add(time.Duration(i+3) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	for _, name := range []string{"2.0", "3.0", "manifest.json", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("retained entry %q was removed: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "1.0")); !os.IsNotExist(err) {
		t.Errorf("old version should be removed, stat error: %v", err)
	}
}

func TestCleanupVersionsDoesNotDeleteUnrelatedFilesWithZeroRetention(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(marker, []byte(`{"version":"3.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if failed := cleanupVersions(root, 0, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != `{"version":"3.0"}` {
		t.Fatalf("unrelated metadata file was changed: %q, %v", data, err)
	}
}
