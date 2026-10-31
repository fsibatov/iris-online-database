//go:build linux

package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// An attacker-controlled special file in the application's data directory
// must not be opened as a JSON document. Opening a FIFO for reading would
// block startup until another process supplied data.
func TestOwnedFileRejectsFIFO(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "profile.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if safeOwnedFilePath(fifo) {
		t.Fatal("FIFO accepted as an owned JSON file")
	}
	if _, err := loadProfileFile(fifo); err == nil {
		t.Fatal("profile loader accepted FIFO")
	}
	if _, err := loadPendingDeleteList(fifo); err == nil {
		t.Fatal("maintenance loader accepted FIFO")
	}
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(backups, "profile.json.bak")
	if err := os.WriteFile(backup, []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"list"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: fifo, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get(); got.Settings.Server != "original" || got.Settings.Theme != "light" {
		t.Fatalf("valid backup not recovered after special-file primary: %+v", got.Settings)
	}
	if err := atomicWriteJSON(fifo, fifo+".bak", defaultProfile()); !errors.Is(err, errUnsafeDestructivePath) {
		t.Fatalf("profile writer accepted FIFO: %v", err)
	}
	regular := filepath.Join(root, "regular.json")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteJSON(regular, fifo, defaultProfile()); !errors.Is(err, errUnsafeDestructivePath) {
		t.Fatalf("backup writer accepted FIFO: %v", err)
	}
}

func TestOwnedFileRejectsUnixSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("Unix socket unavailable: %v", err)
	}
	defer listener.Close()
	if safeOwnedFilePath(path) {
		t.Fatal("Unix socket accepted as an owned JSON file")
	}
	if _, err := loadProfileFile(path); err == nil {
		t.Fatal("profile loader accepted Unix socket")
	}
}

func TestOwnedFileAllowsRegularAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "profile.json")
	if !safeOwnedFilePath(regular) {
		t.Fatal("new regular-file path rejected")
	}
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !safeOwnedFilePath(regular) {
		t.Fatal("existing regular file rejected")
	}
}
