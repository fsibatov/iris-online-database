package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A running executable stored inside Versions is not a rollback candidate.
// Counting it toward keep=2 would discard a recoverable older version.
func TestCleanupVersionsActiveExecutableDoesNotConsumeRetention(t *testing.T) {
	for _, activeVersion := range []string{"1.0", "3.0"} {
		t.Run(activeVersion, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "Versions")
			stamp := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
			makeVersionsAtTimes(t, root, map[string]time.Time{
				"1.0": stamp, "2.0": stamp.Add(time.Hour), "3.0": stamp.Add(2 * time.Hour),
			})
			active := filepath.Join(root, activeVersion, "rollback.bin")
			if failed := cleanupVersions(root, 2, active, log.New(io.Discard, "", 0)); len(failed) != 0 {
				t.Fatalf("active executable reported as failed cleanup: %v", failed)
			}
			checkVersions(t, root, []string{"1.0", "2.0", "3.0"}, nil)
		})
	}
}

func TestCleanupVersionsZeroRetentionPreservesActiveExecutable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{"1.0": stamp, "2.0": stamp})
	active := filepath.Join(root, "2.0", "rollback.bin")
	if failed := cleanupVersions(root, 0, active, log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("active executable should not be scheduled for deletion: %v", failed)
	}
	checkVersions(t, root, []string{"2.0"}, []string{"1.0"})
}

func TestCleanupVersionsExternalExecutableKeepsNormalRetention(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Versions")
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	makeVersionsAtTimes(t, root, map[string]time.Time{"1.0": stamp, "2.0": stamp})
	current := filepath.Join(base, "current.exe")
	if err := os.WriteFile(current, []byte("running"), 0o600); err != nil {
		t.Fatal(err)
	}
	if failed := cleanupVersions(root, 1, current, log.New(io.Discard, "", 0)); len(failed) != 0 {
		t.Fatalf("unexpected cleanup failures: %v", failed)
	}
	checkVersions(t, root, []string{"2.0"}, []string{"1.0"})
	if _, err := os.Stat(current); err != nil {
		t.Fatalf("external executable modified: %v", err)
	}
}
