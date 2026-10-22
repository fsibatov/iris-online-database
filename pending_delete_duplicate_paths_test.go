package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The queue writer deduplicates paths. A disk queue with repeated entries
// cannot be trusted over a valid backup, and must not run the same deletion twice.
func TestPendingDeleteListRejectsDuplicatePaths(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "pending-delete.json")
	entry := filepath.Join(base, "Cache", "obsolete.bin")
	data, err := json.Marshal(pendingDeleteList{Paths: []string{entry, entry}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := loadPendingDeleteList(path); err == nil {
		t.Fatalf("duplicate queue paths accepted: %+v", got.Paths)
	}
}

func TestPendingDeletesRecoverBackupAfterDuplicatePrimary(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "backup-victim.bin")
	if err := os.WriteFile(victim, []byte("obsolete"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "pending-delete.json")
	duplicate := filepath.Join(cache, "missing.bin")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{duplicate, duplicate}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(primary, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup ignored after duplicate primary: %v", err)
	}
}

func TestAppendPendingDeletesRecoversBackupAfterDuplicatePrimary(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	duplicate := filepath.Join(cache, "duplicate.bin")
	for _, path := range []string{old, next, duplicate} {
		if err := os.WriteFile(path, []byte("queued"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(base, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{duplicate, duplicate}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(primary, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(primary)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("queue = %v, %v; expected backup entry and new entry", got.Paths, err)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestPendingDeleteListAllowsDistinctPathsAndLegacyCase(t *testing.T) {
	base := t.TempDir()
	one := filepath.Join(base, "Cache", "one.bin")
	two := filepath.Join(base, "Cache", "two.bin")
	body, err := json.Marshal(map[string]any{"Paths": []string{one, two}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "pending-delete.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(path)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{one, two}) {
		t.Fatalf("distinct legacy paths rejected: %+v, %v", got, err)
	}
}
