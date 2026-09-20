//go:build windows && windows_legacy

package main

import (
	"bytes"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWindowsLegacyRandomAndFilesystem(t *testing.T) {
	first := make([]byte, 64)
	second := make([]byte, 64)
	if _, err := rand.Read(first); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(second); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, make([]byte, len(first))) || bytes.Equal(first, second) {
		t.Fatal("system random generator did not produce independent data")
	}
	root := t.TempDir()
	directory := filepath.Join(root, "Профиль", "Временные файлы")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "данные.txt")
	if err := os.WriteFile(path, first, 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory enumeration: entries=%d error=%v", len(entries), err)
	}
	renamed := filepath.Join(directory, "сохранено.txt")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(renamed); err != nil || !bytes.Equal(data, first) {
		t.Fatalf("file content: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "Профиль")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("directory removal: %v", err)
	}
}

func TestWindowsLegacySubprocess(t *testing.T) {
	if os.Getenv("IRIS_LEGACY_CHILD_PROBE") == "1" {
		if _, err := os.Stdout.WriteString("iris-legacy-child\n"); err != nil {
			t.Fatal(err)
		}
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestWindowsLegacySubprocess$")
	command.Env = append(os.Environ(), "IRIS_LEGACY_CHILD_PROBE=1")
	output, err := command.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("iris-legacy-child\n")) {
		t.Fatalf("child process: %v: %s", err, output)
	}
}
