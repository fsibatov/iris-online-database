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

// Windows cannot open names containing U+0001..U+001F. A queue with such
// paths must not mask the last usable backup or silently drop its entries.
func TestPendingDeleteRejectsWindowsControlCharacters(t *testing.T) {
	for _, bad := range []string{"bad\nname.bin", "bad\tname.bin", "bad\x1fname.bin"} {
		t.Run(bad, func(t *testing.T) {
			root := t.TempDir()
			queue := filepath.Join(root, "pending-delete.json")
			entry := filepath.Join(root, "Cache", bad)
			data, err := json.Marshal(pendingDeleteList{Paths: []string{entry}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(queue, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(queue); err == nil {
				t.Fatalf("unusable Windows path accepted: %+v", got.Paths)
			}
		})
	}
}

func TestPendingDeleteControlCharactersRecoverBackupOnProcess(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "old.bin")
	if err := os.WriteFile(victim, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "invalid\nfile.bin")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(queue, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(queue, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup not replayed after unusable primary: %v", err)
	}
}

func TestPendingDeleteControlCharactersPreserveBackupOnAppend(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
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
	queue := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "bad\tfile.bin")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(queue, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(queue, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("backup queue lost: %+v, %v", got, err)
	}
	preserved, err := os.ReadFile(queue + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestPendingDeleteAllowsUnicodeAndSpacesInNames(t *testing.T) {
	root := t.TempDir()
	queue := filepath.Join(root, "pending-delete.json")
	entry := filepath.Join(root, "Cache", "Обычный файл.bin")
	data, _ := json.Marshal(pendingDeleteList{Paths: []string{entry}})
	if err := os.WriteFile(queue, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{entry}) {
		t.Fatalf("valid Unicode filename rejected: %+v, %v", got, err)
	}
}
