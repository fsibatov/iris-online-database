package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeVersionsAtTimes(t *testing.T, root string, times map[string]time.Time) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, stamp := range times {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "rollback.bin"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
}

func checkVersions(t *testing.T, root string, retained, removed []string) {
	t.Helper()
	for _, name := range retained {
		if _, err := os.Stat(filepath.Join(root, name, "rollback.bin")); err != nil {
			t.Errorf("rollback version %s lost: %v", name, err)
		}
	}
	for _, name := range removed {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("obsolete version %s retained: %v", name, err)
		}
	}
}

// Filesystem timestamps can be identical for versions created in a single
// update batch. Retention must not discard the highest version just because
// ReadDir enumerates the lower names first.
func TestCleanupVersionsEqualTimestampsPreferNewerVersions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{"1.0": stamp, "2.0": stamp, "3.0": stamp})
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"2.0", "3.0"}, []string{"1.0"})
}

// Lexicographic ordering would incorrectly prefer 9.0 over 10.0.
func TestCleanupVersionsEqualTimestampsCompareNumericVersions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{"2.0": stamp, "9.0": stamp, "10.0": stamp})
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"9.0", "10.0"}, []string{"2.0"})
}

// Modification time remains the primary retention criterion, even if a
// lower-numbered rollback folder was updated more recently.
func TestCleanupVersionsDifferentTimestampsRemainPrimary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{"1.0": stamp.Add(2 * time.Hour), "2.0": stamp.Add(time.Hour), "3.0": stamp})
	if failed := cleanupVersions(root, 2, "", log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"1.0", "2.0"}, []string{"3.0"})
}
