package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A corrupt primary queue must not hide the last valid queue: otherwise
// deferred maintenance silently loses work after an interrupted write.
func TestPendingDeletesRecoverValidBackupAfterCorruptPrimary(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "obsolete.bin")
	if err := os.WriteFile(victim, []byte("obsolete"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "keep.bin")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "pending-delete.json")
	if err := os.WriteFile(primary, []byte(`{"paths":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := json.Marshal(pendingDeleteList{Paths: []string{victim, outside}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}

	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup queue was ignored; obsolete file remains: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was modified: %v", err)
	}
}

// Appending to a damaged queue must recover previous entries and preserve the
// valid backup instead of replacing it with malformed primary JSON.
func TestAppendPendingDeletesPreservesBackupAfterCorruptPrimary(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(base, "pending-delete.json")
	if err := os.WriteFile(primary, []byte(`{"paths":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", original, 0o600); err != nil {
		t.Fatal(err)
	}

	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	data, err := os.ReadFile(primary)
	if err != nil {
		t.Fatal(err)
	}
	var got pendingDeleteList
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Errorf("recovered pending paths = %v, want both old and new", got.Paths)
	}
	savedBackup, err := os.ReadFile(primary + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(savedBackup, original) {
		t.Errorf("valid pending-delete backup was overwritten by damaged primary")
	}
}

// A missing primary may indicate that maintenance completed successfully.
// An older .bak must not be replayed in that case.
func TestPendingDeletesDoNotReplayStaleBackupWhenPrimaryMissing(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "reused.bin")
	if err := os.WriteFile(old, []byte("new content"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "pending-delete.json")
	original, err := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", original, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("stale backup replayed after successful cleanup: %v", err)
	}
}
