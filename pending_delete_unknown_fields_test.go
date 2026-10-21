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

// Unknown queue envelope fields can carry future safety semantics. An older
// binary must not silently ignore them and execute destructive operations.
func TestPendingDeleteRejectsUnknownEnvelopeFields(t *testing.T) {
	for _, body := range []string{
		`{"paths":[],"requiresConfirmation":true}`,
		`{"Paths":[],"version":2}`,
	} {
		path := filepath.Join(t.TempDir(), "pending-delete.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if list, err := loadPendingDeleteList(path); err == nil {
			t.Fatalf("unknown maintenance field accepted: %+v", list)
		}
	}
}

func TestPendingDeleteUnknownPrimaryUsesSafeBackup(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	primaryVictim := filepath.Join(cache, "must-not-delete.bin")
	backupVictim := filepath.Join(cache, "obsolete.bin")
	for _, path := range []string{primaryVictim, backupVictim} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(map[string]any{"paths": []string{primaryVictim}, "requiresConfirmation": true})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{backupVictim}})
	if err := os.WriteFile(primary, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(primaryVictim); err != nil {
		t.Fatalf("executed unknown queue semantics, deleted protected primary target: %v", err)
	}
	if _, err := os.Stat(backupVictim); !os.IsNotExist(err) {
		t.Fatalf("failed to replay known-good backup queue: %v", err)
	}
}

func TestAppendPendingDeletesPreservesBackupFromUnknownPrimary(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	unknown := filepath.Join(cache, "unknown.bin")
	for _, path := range []string{old, next, unknown} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(map[string]any{"paths": []string{unknown}, "requiresConfirmation": true})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(primary, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(primary)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("pending paths = %v; want preserved backup plus new entry", got.Paths)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preserved, good) {
		t.Fatal("known-good backup overwritten by unknown primary")
	}
}

func TestPendingDeleteAcceptsOnlyKnownPathsAndLegacyAlias(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pending-delete.json")
	entry := filepath.Join(root, "Cache", "old.bin")
	for _, body := range []string{
		`{"paths":[]}`,
		`{"Paths":[` + mustJSONQuote(entry) + `]}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadPendingDeleteList(path); err != nil {
			t.Fatalf("valid legacy queue rejected: %v", err)
		}
	}
}

func mustJSONQuote(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
