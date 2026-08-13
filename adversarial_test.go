package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestDestructivePathNormalInsideRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Cache")
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := safeRemove(file, root, ""); err != nil {
		t.Fatalf("safeRemove: %v", err)
	}
	if err := safeRemoveAll(filepath.Join(root, "dir"), root, ""); err != nil {
		t.Fatalf("safeRemoveAll: %v", err)
	}
}

func TestDestructivePathRejectsOutsideTraversalRootAndExecutable(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Cache")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates := []string{victim, filepath.Join(root, "..", "outside", "victim.txt"), root, ""}
	for _, candidate := range candidates {
		if err := safeRemoveAll(candidate, root, ""); err == nil {
			t.Fatalf("unsafe path unexpectedly accepted: %q", candidate)
		}
	}
	exe := filepath.Join(root, "IrisOnlineDB.exe")
	if err := os.WriteFile(exe, []byte("exe"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := safeRemove(exe, root, exe); err == nil {
		t.Fatal("current executable was accepted for deletion")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("outside victim was modified")
	}
}

func TestDestructivePathRejectsSymlinkFinalAndAncestor(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")
	victimDir := filepath.Join(outside, "victim")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(victimDir, 0o700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(victimDir, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := safeRemoveAll(filepath.Join(link, "victim"), root, ""); err == nil {
		t.Fatal("symlink ancestor escape was accepted")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("outside victim was deleted through symlink ancestor")
	}

	finalLink := filepath.Join(root, "final-link")
	if err := os.Symlink(keep, finalLink); err != nil {
		t.Fatal(err)
	}
	if err := safeRemoveAll(finalLink, root, ""); err == nil {
		t.Fatal("final symlink was accepted")
	}
	broken := filepath.Join(root, "broken-link")
	if err := os.Symlink(filepath.Join(outside, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	if err := safeRemoveAll(broken, root, ""); err == nil {
		t.Fatal("broken symlink was accepted")
	}
}

func TestPendingDeleteCannotEscapeThroughSymlinkAncestor(t *testing.T) {
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
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	listPath := filepath.Join(base, "pending-delete.json")
	if err := os.WriteFile(listPath, []byte(fmt.Sprintf(`{"paths":[%q]}`, filepath.Join(link, "victim"))), 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(listPath, []string{root}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("outside/victim/keep.txt was deleted by pending maintenance")
	}
}

func TestPendingDeleteMalformedAndProfileOutsideRootAreSafe(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Cache")
	profileDir := filepath.Join(base, "UserData")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(profileDir, "profile.json")
	if err := os.WriteFile(profile, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	listPath := filepath.Join(base, "pending-delete.json")
	if err := os.WriteFile(listPath, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(listPath, []string{root}, "", log.New(io.Discard, "", 0))
	if err := safeRemoveAll(profile, root, ""); err == nil {
		t.Fatal("profile outside maintenance root was accepted")
	}
	if _, err := os.Stat(profile); err != nil {
		t.Fatal("profile was modified by maintenance")
	}
}

func TestDestructivePathRejectsNestedSymlinkChainAndSymlinkRoot(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real-root")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(realRoot, "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "victim"), 0o700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(outside, "victim", "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(realRoot, "second")
	first := filepath.Join(realRoot, "first")
	if err := os.Symlink(outside, second); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.Symlink(second, first); err != nil {
		t.Fatal(err)
	}
	if err := safeRemoveAll(filepath.Join(first, "victim"), realRoot, ""); err == nil {
		t.Fatal("nested symlink chain was accepted")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("outside victim changed through nested symlink chain")
	}

	rootLink := filepath.Join(base, "root-link")
	if err := os.Symlink(realRoot, rootLink); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(realRoot, "inner", "plain.txt")
	if err := os.WriteFile(plain, []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := safeRemove(filepath.Join(rootLink, "inner", "plain.txt"), rootLink, ""); err == nil {
		t.Fatal("symlinked maintenance root was accepted even though it resolves to an owned sibling")
	}
	if _, err := os.Stat(plain); err != nil {
		t.Fatal("file under real root changed through symlinked root")
	}
}

func TestDestructivePathRejectsDirectoryContainingExecutable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Versions")
	version := filepath.Join(root, "1.0")
	if err := os.MkdirAll(version, 0o700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(version, "IrisOnlineDB.exe")
	if err := os.WriteFile(exe, []byte("exe"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := safeRemoveAll(version, root, exe); err == nil {
		t.Fatal("directory containing current executable was accepted for recursive deletion")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatal("current executable changed")
	}
}

func TestAtomicProfileBackupCannotFollowSymlink(t *testing.T) {
	base := t.TempDir()
	userData := filepath.Join(base, "UserData")
	backups := filepath.Join(userData, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(userData, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	outside := filepath.Join(base, "outside.txt")
	if err := os.WriteFile(profile, []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, backup); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := atomicWriteJSON(profile, backup, map[string]any{"schemaVersion": 1}); err == nil {
		t.Fatal("profile write accepted a symlink backup path")
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "keep" {
		t.Fatalf("outside backup target changed: %q err=%v", got, err)
	}
}

func TestConcurrentMaintenanceCannotEscapeRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Cache")
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
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	plain := filepath.Join(root, "old.tmp")
	if err := os.WriteFile(plain, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(plain, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	logger := log.New(io.Discard, "", 0)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		listPath := filepath.Join(base, fmt.Sprintf("pending-%d.json", i))
		if err := os.WriteFile(listPath, []byte(fmt.Sprintf(`{"paths":[%q]}`, filepath.Join(link, "victim"))), 0o600); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func(listPath string) {
			defer wg.Done()
			processPendingDeletes(listPath, []string{root}, "", logger)
			_ = cleanupDirectory(root, time.Hour, 0, "", logger)
		}(listPath)
	}
	wg.Wait()
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("concurrent maintenance changed outside victim: %v", err)
	}
}

func TestPendingDeleteListSymlinkIsIgnored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require Windows developer mode; reparse paths are covered separately")
	}
	base := t.TempDir()
	root := filepath.Join(base, "Cache")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(root, "victim")
	if err := os.MkdirAll(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	outsideList := filepath.Join(t.TempDir(), "outside-pending.json")
	payload := []byte(fmt.Sprintf(`{"paths":[%q]}`, victim))
	if err := os.WriteFile(outsideList, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	listPath := filepath.Join(base, "pending-delete.json")
	if err := os.Symlink(outsideList, listPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	processPendingDeletes(listPath, []string{root}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("symlinked pending list triggered deletion: %v", err)
	}
	data, err := os.ReadFile(outsideList)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("outside pending list was modified: %q", data)
	}
}
