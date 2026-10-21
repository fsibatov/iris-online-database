package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A NUL byte is representable in JSON but cannot occur in an OS pathname.
// Such a primary must not hide a usable backup queue.
func TestPendingDeleteListRejectsEmbeddedNUL(t *testing.T) {
	base := t.TempDir()
	for _, suffix := range []string{"bad\x00name.bin", "\x00bad.bin"} {
		path := filepath.Join(base, "pending-delete.json")
		invalid := filepath.Join(base, "Cache", suffix)
		if !filepath.IsAbs(invalid) || filepath.Clean(invalid) != invalid || !strings.ContainsRune(invalid, 0) {
			t.Fatalf("invalid test path: %q", invalid)
		}
		data, err := json.Marshal(pendingDeleteList{Paths: []string{invalid}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := loadPendingDeleteList(path); err == nil {
			t.Fatalf("NUL pathname accepted: %+v", got)
		}
	}
}

func TestPendingDeletesRecoverBackupFromNULPrimary(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "obsolete.bin")
	if err := os.WriteFile(victim, []byte("obsolete"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "pending-delete.json")
	badData, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "bad\x00name.bin")}})
	if err := os.WriteFile(primary, badData, 0o600); err != nil {
		t.Fatal(err)
	}
	backup, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup ignored after NUL primary: %v", err)
	}
}

func TestAppendPendingDeletesRecoversBackupFromNULPrimary(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("queued"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(base, "pending-delete.json")
	badData, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "bad\x00name.bin")}})
	if err := os.WriteFile(primary, badData, 0o600); err != nil {
		t.Fatal(err)
	}
	backup, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(primary)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("queue = %q, want both old and new", got.Paths)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !reflect.DeepEqual(preserved, backup) {
		t.Fatalf("valid backup changed: %q, %v", preserved, err)
	}
}

func TestPendingDeleteListAcceptsOrdinaryCanonicalPath(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "pending-delete.json")
	entry := filepath.Join(base, "Cache", "normal file.bin")
	data, _ := json.Marshal(pendingDeleteList{Paths: []string{entry}})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(path)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{entry}) {
		t.Fatalf("valid path rejected: %+v, %v", got, err)
	}
}
