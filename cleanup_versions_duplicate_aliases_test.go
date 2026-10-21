package main

import (
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"
)

// Different accepted spellings of one version must not consume multiple
// rollback slots and evict a different, recoverable version.
func TestCleanupVersionsDuplicateAliasesDoNotEvictDistinctRollback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-4 * time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{
		"1.0":  stamp,
		"2.0":  stamp.Add(time.Hour),
		"3.0":  stamp.Add(2 * time.Hour),
		"v3.0": stamp.Add(3 * time.Hour),
	})
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"2.0", "v3.0"}, []string{"1.0", "3.0"})
}

func TestCleanupVersionsNumericallyEquivalentAliasesDoNotEvictDistinctRollback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-4 * time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{
		"1.0":  stamp,
		"2.0":  stamp.Add(time.Hour),
		"02.0": stamp.Add(2 * time.Hour),
	})
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"1.0", "02.0"}, []string{"2.0"})
}

func TestCleanupVersionsDuplicateAliasesZeroRetention(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{
		"2.0":     stamp,
		"v2.0":    stamp,
		"staging": stamp,
	})
	if failed := cleanupVersions(root, 0, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"staging"}, []string{"2.0", "v2.0"})
}
