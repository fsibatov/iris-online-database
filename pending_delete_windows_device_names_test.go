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

func TestPendingDeleteRejectsWindowsDeviceNames(t *testing.T) {
	for _, name := range []string{"CON", "con.txt", "PRN", "AUX.bin", "NUL", "COM1", "com9.log", "COM¹.log", "LPT1", "lpt9.txt", "LPT².txt", "folder/CON.txt"} {
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
				t.Fatalf("Win32 device pathname accepted: %+v", got.Paths)
			}
		})
	}
}

func TestPendingDeleteWindowsDeviceNameRecoversBackup(t *testing.T) {
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
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "CON.txt")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(queue, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(queue, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("backup deletion not replayed: %v", err)
	}
}

func TestAppendPendingDeletesRecoversWindowsDeviceNameBackup(t *testing.T) {
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
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "LPT1.log")}})
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
		t.Fatalf("valid backup lost: %+v %v", got, err)
	}
	preserved, err := os.ReadFile(queue + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("backup overwritten: %q %v", preserved, err)
	}
}

func TestPendingDeleteAllowsNonDeviceNames(t *testing.T) {
	root := t.TempDir()
	queue := filepath.Join(root, "pending-delete.json")
	paths := []string{filepath.Join(root, "Cache", "CONSOLE.txt"), filepath.Join(root, "Cache", "COM10"), filepath.Join(root, "Cache", "prefix.CON")}
	data, _ := json.Marshal(pendingDeleteList{Paths: paths})
	if err := os.WriteFile(queue, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, paths) {
		t.Fatalf("valid paths rejected: %+v %v", got, err)
	}
}
