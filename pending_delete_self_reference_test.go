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

// The maintenance queue must never schedule deletion of its own metadata.
// Such a primary is unusable and must not conceal an intact backup.
func TestPendingDeleteRejectsSelfReferentialMetadata(t *testing.T) {
	for _, suffix := range []string{"", ".bak"} {
		t.Run("suffix="+suffix, func(t *testing.T) {
			primary := filepath.Join(t.TempDir(), "pending-delete.json")
			bad, _ := json.Marshal(pendingDeleteList{Paths: []string{primary + suffix}})
			if err := os.WriteFile(primary, bad, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(primary); err == nil {
				t.Fatalf("self-referential queue accepted: %+v", got)
			}
		})
	}
}

func TestPendingDeleteSelfReferenceRestoresBackupForProcessing(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "obsolete.bin")
	if err := os.WriteFile(victim, []byte("obsolete"), 0600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{primary}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(primary, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("backup cleanup not replayed: %v", err)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup changed: %q, %v", preserved, err)
	}
}

func TestPendingDeleteSelfReferenceRestoresBackupForAppend(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "new.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("pending"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{primary + ".bak"}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(primary, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(primary)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("lost backup path: %+v, %v", got, err)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestPendingDeleteAllowsOrdinaryQueueAndLegacyCase(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "pending-delete.json")
	entry := filepath.Join(root, "Cache", "obsolete.bin")
	body, _ := json.Marshal(map[string]any{"Paths": []string{entry}})
	if err := os.WriteFile(primary, body, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(primary)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{entry}) {
		t.Fatalf("ordinary legacy queue rejected: %+v, %v", got, err)
	}
}
