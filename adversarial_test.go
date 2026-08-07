package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func expireSessionForTest(t *testing.T, sessions *sessionManager, id string) {
	t.Helper()
	sessions.mu.Lock()
	if _, ok := sessions.sessions[id]; !ok {
		sessions.mu.Unlock()
		t.Fatalf("session %q is not active", id)
	}
	sessions.sessions[id] = time.Now().Add(-sessions.heartbeatTimeout - time.Second)
	sessions.mu.Unlock()
	_ = sessions.ActiveCount(time.Now())
}

func TestExpiredSessionDirectCloseShutsDown(t *testing.T) {
	s := newSessionManager(time.Millisecond, 5*time.Millisecond, time.Minute)
	id := s.Open("expired-session-0001")
	expireSessionForTest(t, s, id)
	if s.ExpiredCount(time.Now()) != 1 {
		t.Fatal("expired session tombstone was not retained")
	}
	if s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
		t.Fatal("heartbeat expiry alone must not stop normal browser mode")
	}
	s.Close(id)
	if !s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
		t.Fatal("explicit close of a recently expired session did not allow shutdown")
	}
}

func TestUnknownSessionCloseCannotShutDown(t *testing.T) {
	s := newSessionManager(time.Millisecond, 5*time.Millisecond, time.Minute)
	id := s.Open("known-session-000001")
	expireSessionForTest(t, s, id)
	s.Close("random-unknown-0001")
	if s.ExpiredCount(time.Now()) != 1 {
		t.Fatal("unknown close modified expired-session state")
	}
	if s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
		t.Fatal("unknown session ID was able to stop the application")
	}
}

func TestExpiredSessionReopenThenClose(t *testing.T) {
	s := newSessionManager(time.Millisecond, 5*time.Millisecond, time.Minute)
	id := s.Open("reopen-session-00001")
	expireSessionForTest(t, s, id)
	if got := s.Open(id); got != id {
		t.Fatalf("reopen returned %q want %q", got, id)
	}
	if s.ExpiredCount(time.Now()) != 0 || s.ActiveCount(time.Now()) != 1 {
		t.Fatal("reopen did not move tombstone back to active sessions")
	}
	s.Close(id)
	if !s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
		t.Fatal("reopened session close did not allow shutdown")
	}
}

func TestTwoSessionExpiryCloseOrders(t *testing.T) {
	t.Run("expired closes while other active", func(t *testing.T) {
		s := newSessionManager(time.Millisecond, 5*time.Millisecond, time.Minute)
		a := s.Open("two-session-A-0001")
		b := s.Open("two-session-B-0001")
		expireSessionForTest(t, s, a)
		s.Close(a)
		if s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
			t.Fatal("application would stop while session B is active")
		}
		s.Close(b)
		if !s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
			t.Fatal("last confirmed session close did not allow shutdown")
		}
	})

	t.Run("active closes while expired remains", func(t *testing.T) {
		s := newSessionManager(time.Millisecond, 5*time.Millisecond, time.Minute)
		a := s.Open("two-session-A-0002")
		b := s.Open("two-session-B-0002")
		expireSessionForTest(t, s, a)
		s.Close(b)
		if s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
			t.Fatal("application would stop while a confirmed expired session may still be a frozen tab")
		}
		s.Close(a)
		if !s.ShouldShutdown(time.Now().Add(10 * time.Millisecond)) {
			t.Fatal("closing the remaining expired session did not allow shutdown")
		}
	})
}

func TestExpiredSessionTombstonesBoundedAndExpire(t *testing.T) {
	s := newSessionManager(time.Millisecond, 5*time.Millisecond, 5*time.Millisecond)
	now := time.Now()
	s.mu.Lock()
	for i := 0; i < maxExpiredSessionCount+100; i++ {
		s.addExpiredLocked(fmt.Sprintf("bounded-session-%08d", i), now)
	}
	count := len(s.expiredSessions)
	s.mu.Unlock()
	if count != maxExpiredSessionCount {
		t.Fatalf("expired tombstones=%d want hard limit %d", count, maxExpiredSessionCount)
	}

	id := s.Open("ttl-session-0000001")
	expireSessionForTest(t, s, id)
	future := time.Now().Add(20 * time.Millisecond)
	if got := s.ExpiredCount(future); got != 0 {
		t.Fatalf("expired tombstones after TTL=%d want 0", got)
	}
	if !s.ShouldShutdown(future.Add(10 * time.Millisecond)) {
		t.Fatal("abandoned tombstone expiry would leave the backend running forever")
	}
}

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

func TestSessionManagerConcurrentLifecycleRemainsBounded(t *testing.T) {
	s := newSessionManager(time.Millisecond, 2*time.Millisecond, 25*time.Millisecond)
	const workers = 12
	const iterations = 150
	var wg sync.WaitGroup
	wg.Add(workers + 1)
	for worker := 0; worker < workers; worker++ {
		worker := worker
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := fmt.Sprintf("concurrent-session-%02d-%04d", worker, i%40)
				s.Open(id)
				_ = s.Heartbeat(id)
				if i%3 == 0 {
					s.Close(id)
				}
				_ = s.ActiveCount(time.Now())
				_ = s.ExpiredCount(time.Now())
				_ = s.ShouldShutdown(time.Now())
			}
		}()
	}
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			s.mu.Lock()
			for id := range s.sessions {
				s.sessions[id] = time.Now().Add(-time.Second)
				break
			}
			s.mu.Unlock()
			_ = s.ActiveCount(time.Now())
			time.Sleep(50 * time.Microsecond)
		}
	}()
	wg.Wait()

	if got := s.ActiveCount(time.Now()); got > maxSessionCount {
		t.Fatalf("active sessions exceeded cap: %d > %d", got, maxSessionCount)
	}
	if got := s.ExpiredCount(time.Now()); got > maxExpiredSessionCount {
		t.Fatalf("expired-session tombstones exceeded cap: %d > %d", got, maxExpiredSessionCount)
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
