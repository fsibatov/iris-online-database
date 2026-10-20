package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// atomicWriteJSON also persists maintenance metadata. Its profile-specific
// corruption guard must not disable backups for other valid JSON payloads.
func TestAtomicWriteJSONBacksUpPendingDeleteList(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pending-delete.json")
	backup := path + ".bak"
	previous := pendingDeleteList{Paths: []string{"old-cache-entry"}}
	current := pendingDeleteList{Paths: []string{"new-cache-entry"}}
	if err := atomicWriteJSON(path, backup, previous); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteJSON(path, backup, current); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("previous pending-delete list was not backed up: %v", err)
	}
	var got pendingDeleteList
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, previous) {
		t.Fatalf("backup = %+v, want %+v", got, previous)
	}
}

func TestAtomicWriteJSONBacksUpOtherJSONPayloads(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "metadata.json")
	backup := path + ".bak"
	if err := atomicWriteJSON(path, backup, map[string]string{"state": "previous"}); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteJSON(path, backup, map[string]string{"state": "current"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("previous generic JSON payload was not backed up: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["state"] != "previous" {
		t.Fatalf("backup = %v, want previous state", got)
	}
}
