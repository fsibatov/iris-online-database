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

// CONIN$ and CONOUT$ are reserved Win32 console devices, not ordinary files.
// An invalid primary queue must not hide a recoverable backup queue.
func TestPendingDeleteRejectsWindowsConsoleDeviceNames(t *testing.T) {
	for _, name := range []string{"CONIN$", "conin$", "CONOUT$", "conout$", "folder/CONIN$"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			queue := filepath.Join(root, "pending-delete.json")
			candidate := filepath.Join(root, "Cache", filepath.FromSlash(name))
			data, err := json.Marshal(pendingDeleteList{Paths: []string{candidate}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(queue, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(queue); err == nil {
				t.Fatalf("reserved Win32 console device accepted: %+v", got.Paths)
			}
		})
	}
}

func TestPendingDeleteConsoleDevicePrimaryRecoversBackup(t *testing.T) {
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
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "CONOUT$")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(queue, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(queue, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("backup queue not replayed: %v", err)
	}
}

func TestAppendPendingDeletesPreservesBackupForConsoleDevice(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old, next := filepath.Join(cache, "old.bin"), filepath.Join(cache, "next.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	queue := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "CONIN$")}})
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
		t.Fatalf("valid backup queue lost: %+v, %v", got, err)
	}
	preserved, err := os.ReadFile(queue + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestPendingDeleteAllowsNonConsoleDeviceNames(t *testing.T) {
	root := t.TempDir()
	queue := filepath.Join(root, "pending-delete.json")
	paths := []string{
		filepath.Join(root, "Cache", "CONIN$X"),
		filepath.Join(root, "Cache", "CONOUT$-notes"),
	}
	data, _ := json.Marshal(pendingDeleteList{Paths: paths})
	if err := os.WriteFile(queue, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, paths) {
		t.Fatalf("safe names rejected: %+v, %v", got, err)
	}
}
