//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This is a native Windows regression for the exact junction-ancestor escape
// shape that lexical filepath.Rel checks cannot detect. It is Windows-only and
// is not executed by the Linux packaging environment.
func TestWindowsJunctionAncestorCannotEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")
	victim := filepath.Join(outside, "victim")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(victim, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if output, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("cannot create Windows junction: %v (%s)", err, output)
	}
	defer exec.Command("cmd.exe", "/d", "/c", "rmdir", link).Run()

	if err := safeRemoveAll(filepath.Join(link, "victim"), root, ""); err == nil {
		t.Fatal("junction ancestor escape was accepted")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("outside victim was changed through junction: %v", err)
	}
}
