package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The maintenance writer stores cleaned absolute paths. An unclean primary
// cannot take precedence over a valid backup or overwrite it during append.
func TestPendingDeleteListRejectsNonCanonicalPaths(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	for _, tc := range []struct{ name, path string }{
		{"dot-segment", cache + string(filepath.Separator) + "." + string(filepath.Separator) + "obsolete.bin"},
		{"parent-segment", cache + string(filepath.Separator) + "tmp" + string(filepath.Separator) + ".." + string(filepath.Separator) + "obsolete.bin"},
		{"duplicate-separator", cache + string(filepath.Separator) + string(filepath.Separator) + "obsolete.bin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !filepath.IsAbs(tc.path) || filepath.Clean(tc.path) == tc.path {
				t.Fatalf("bad test fixture: %q", tc.path)
			}
			path := filepath.Join(base, tc.name+".json")
			data, err := json.Marshal(pendingDeleteList{Paths: []string{tc.path}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(path); err == nil {
				t.Fatalf("non-canonical path accepted: %+v", got)
			}
		})
	}
}

func TestPendingDeletesRecoverBackupFromNonCanonicalPrimary(t *testing.T) {
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
	bad := cache + string(filepath.Separator) + "." + string(filepath.Separator) + "missing.bin"
	badData, _ := json.Marshal(pendingDeleteList{Paths: []string{bad}})
	if err := os.WriteFile(primary, badData, 0o600); err != nil {
		t.Fatal(err)
	}
	backup, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup ignored after non-canonical primary: %v", err)
	}
}

func TestAppendPendingDeletesPreservesBackupAfterNonCanonicalPrimary(t *testing.T) {
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
	bad := cache + string(filepath.Separator) + "." + string(filepath.Separator) + "missing.bin"
	badData, _ := json.Marshal(pendingDeleteList{Paths: []string{bad}})
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
		t.Fatalf("queue = %q, want old and new entries", got.Paths)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !reflect.DeepEqual(preserved, backup) {
		t.Fatalf("valid backup changed: %s, %v", preserved, err)
	}
}

func TestPendingDeleteListAcceptsCanonicalPathWithSpaces(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "pending-delete.json")
	entry := filepath.Join(base, "Cache", "valid file.bin")
	data, _ := json.Marshal(pendingDeleteList{Paths: []string{entry}})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(path)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{entry}) {
		t.Fatalf("valid canonical path rejected: %+v, %v", got, err)
	}
}
