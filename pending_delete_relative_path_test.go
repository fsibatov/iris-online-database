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

// The maintenance writer persists absolute paths. A relative path in the
// primary queue is corruption, not a valid queue that may hide its backup.
func TestPendingDeleteListRejectsRelativePaths(t *testing.T) {
	for _, relative := range []string{"obsolete.bin", filepath.Join("Cache", "obsolete.bin"), ".."} {
		t.Run(relative, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pending-delete.json")
			data, err := json.Marshal(pendingDeleteList{Paths: []string{relative}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(path); err == nil {
				t.Fatalf("relative maintenance path accepted: %+v", got)
			}
		})
	}
}

func TestPendingDeletesRecoverBackupFromRelativePrimary(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "obsolete.bin")
	if err := os.WriteFile(victim, []byte("obsolete"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "pending-delete.json")
	if err := os.WriteFile(primary, []byte(`{"paths":["relative.bin"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup ignored after relative primary: %v", err)
	}
}

func TestAppendPendingDeletesRecoversBackupFromRelativePrimary(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("queued"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(root, "pending-delete.json")
	if err := os.WriteFile(primary, []byte(`{"paths":["relative.bin"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	list, err := loadPendingDeleteList(primary)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list.Paths, []string{old, next}) {
		t.Fatalf("queue = %q; want recovered and appended paths", list.Paths)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !reflect.DeepEqual(preserved, backup) {
		t.Fatalf("valid backup overwritten: %s, %v", preserved, err)
	}
}
