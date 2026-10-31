package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The queue writer must not publish JSON that loadPendingDeleteList will
// refuse at the next startup because it exceeds maxPendingListBytes.
func oversizedPendingDeleteQueue(root string) pendingDeleteList {
	paths := make([]string, 14000)
	for i := range paths {
		paths[i] = filepath.Join(root, fmt.Sprintf("entry-%05d-%s", i, strings.Repeat("x", 48)))
	}
	return pendingDeleteList{Paths: paths}
}

func assertOversizedPendingDeleteFixture(t *testing.T, queue pendingDeleteList) {
	t.Helper()
	encoded, err := json.MarshalIndent(queue, "", "  ")
	if err != nil || int64(len(encoded)) <= maxPendingListBytes {
		t.Fatalf("invalid oversized queue fixture: size=%d, err=%v", len(encoded), err)
	}
}

func TestPendingDeleteWriteRejectsUnreloadableOversizedQueue(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "pending-delete.json")
	backup := primary + ".bak"
	previous := pendingDeleteList{Paths: []string{filepath.Join(root, "Cache", "first")}}
	current := pendingDeleteList{Paths: []string{filepath.Join(root, "Cache", "second")}}
	for _, queue := range []pendingDeleteList{previous, current} {
		if err := atomicWriteJSON(primary, backup, queue); err != nil {
			t.Fatal(err)
		}
	}
	oldPrimary, err := os.ReadFile(primary)
	if err != nil {
		t.Fatal(err)
	}
	oldBackup, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	large := oversizedPendingDeleteQueue(root)
	assertOversizedPendingDeleteFixture(t, large)
	if err := atomicWriteJSON(primary, backup, large); err == nil {
		t.Fatal("writer accepted a queue that loadPendingDeleteList cannot read")
	}
	for path, want := range map[string][]byte{primary: oldPrimary, backup: oldBackup} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("rejected write changed %s: %v", path, err)
		}
	}
	if got, err := loadPendingDeleteList(primary); err != nil || len(got.Paths) != 1 || got.Paths[0] != current.Paths[0] {
		t.Fatalf("existing queue was lost: %+v, %v", got, err)
	}
}

func TestPendingDeleteFirstWriteRejectsUnreloadableOversizedQueue(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "pending-delete.json")
	large := oversizedPendingDeleteQueue(root)
	assertOversizedPendingDeleteFixture(t, large)
	if err := atomicWriteJSON(primary, primary+".bak", &large); err == nil {
		t.Fatal("writer accepted an unreadable first queue")
	}
	for _, path := range []string{primary, primary + ".bak", primary + ".old"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("rejected first write left %s: %v", path, err)
		}
	}
}

func TestPendingDeleteWriteBelowLimitRemainsReadable(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "pending-delete.json")
	queue := pendingDeleteList{Paths: []string{filepath.Join(root, "Cache", "ordinary entry")}}
	if err := atomicWriteJSON(primary, primary+".bak", queue); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(primary)
	if err != nil || len(got.Paths) != 1 || got.Paths[0] != queue.Paths[0] {
		t.Fatalf("valid queue was not reloadable: %+v, %v", got, err)
	}
}
