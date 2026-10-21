package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Only rollback-version directories are eligible for retention and deletion.
// Staging/metadata directories must neither evict a valid rollback nor be removed.
func TestCleanupVersionsIgnoresUnrelatedDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-4 * time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{
		"1.0": stamp, "2.0": stamp.Add(time.Hour), "3.0": stamp.Add(2 * time.Hour),
		"staging": stamp.Add(3 * time.Hour),
	})
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"2.0", "3.0", "staging"}, []string{"1.0"})
}

func TestCleanupVersionsZeroRetentionPreservesUnrelatedDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{
		"2.0.5": stamp, "metadata": stamp,
	})
	if failed := cleanupVersions(root, 0, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"metadata"}, []string{"2.0.5"})
}

func TestCleanupVersionsAcceptsPrefixedSemanticVersions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{
		"v2.0.4": stamp, "v2.0.5": stamp,
	})
	if failed := cleanupVersions(root, 1, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	if _, err := os.Stat(filepath.Join(root, "v2.0.5", "rollback.bin")); err != nil {
		t.Fatalf("valid prefixed version removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "v2.0.4")); !os.IsNotExist(err) {
		t.Fatalf("older prefixed version not removed: %v", err)
	}
}
