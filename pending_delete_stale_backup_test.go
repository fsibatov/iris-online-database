package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
)

// A completed queue must not become the fallback for a newly created queue.
func TestPendingDeletesDoNotReplayPreviousGenerationAfterCorruption(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	reused := filepath.Join(cache, "reused.bin")
	next := filepath.Join(cache, "next.bin")
	if err := os.WriteFile(reused, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "pending-delete.json")
	previous, err := json.Marshal(pendingDeleteList{Paths: []string{reused}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", previous, 0o600); err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard, "", 0)
	processPendingDeletes(primary, []string{cache}, "", logger)
	if _, err := os.Stat(primary); !os.IsNotExist(err) {
		t.Fatalf("completed queue not removed: %v", err)
	}
	if _, err := os.Stat(reused); !os.IsNotExist(err) {
		t.Fatalf("old file not removed: %v", err)
	}
	if err := os.WriteFile(reused, []byte("new user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte("pending"), 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", logger)
	list, err := loadPendingDeleteList(primary)
	if err != nil || len(list.Paths) != 1 || list.Paths[0] != next {
		t.Fatalf("new queue not saved: %+v, %v", list, err)
	}
	if err := os.WriteFile(primary, []byte(`{"paths":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", logger)
	if content, err := os.ReadFile(reused); err != nil || string(content) != "new user data" {
		t.Fatalf("stale backup deleted newly created file: content=%q, err=%v", content, err)
	}
}

func TestNewPendingDeleteGenerationInvalidatesStaleBackup(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	for _, p := range []string{old, next} {
		if err := os.WriteFile(p, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(base, "pending-delete.json")
	stale, err := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", stale, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(primary + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("old generation backup remains after creating new queue: %v", err)
	}
	list, err := loadPendingDeleteList(primary)
	if err != nil || len(list.Paths) != 1 || list.Paths[0] != next {
		t.Fatalf("new queue incorrect: %+v, %v", list, err)
	}
}
