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

func TestPendingDeleteRejectsCaseFoldedDuplicatePaths(t *testing.T) {
	for _, names := range [][2]string{{"Archive.DAT", "archive.dat"}, {"Keep.bin", "keep.bin"}} {
		t.Run(names[0], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pending-delete.json")
			cache := filepath.Join(filepath.Dir(path), "Cache")
			body, err := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, names[0]), filepath.Join(cache, names[1])}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(path); err == nil {
				t.Fatalf("case-folded duplicate queue accepted: %+v", got)
			}
		})
	}
}

func TestPendingDeleteCaseFoldedDuplicatesPreserveBackupOnAppend(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "Old.bin")
	next := filepath.Join(cache, "New.bin")
	for _, name := range []string{old, next} {
		if err := os.WriteFile(name, []byte("pending"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{old, filepath.Join(cache, "old.bin")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(primary, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", good, 0600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(primary)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("valid backup queue not recovered: %+v, %v", got, err)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestPendingDeleteAcceptsDistinctPathsAfterCaseFold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending-delete.json")
	cache := filepath.Join(filepath.Dir(path), "Cache")
	paths := []string{filepath.Join(cache, "First.bin"), filepath.Join(cache, "Second.bin")}
	body, _ := json.Marshal(pendingDeleteList{Paths: paths})
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(path)
	if err != nil || !reflect.DeepEqual(got.Paths, paths) {
		t.Fatalf("distinct paths rejected: %+v, %v", got, err)
	}
}

func TestPendingDeleteCaseFoldAllowsLegacyEnvelope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending-delete.json")
	entry := filepath.Join(filepath.Dir(path), "Cache", "Single.bin")
	body, _ := json.Marshal(map[string]any{"Paths": []string{entry}})
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(path)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{entry}) {
		t.Fatalf("legacy queue rejected: %+v, %v", got, err)
	}
}

func TestPendingDeleteAppendDeduplicatesCaseFoldedPaths(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(cache, "Old.bin")
	alias := filepath.Join(cache, "old.bin")
	for _, path := range []string{first, alias} {
		if err := os.WriteFile(path, []byte("pending"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	queue := filepath.Join(root, "pending-delete.json")
	appendPendingDeletes(queue, []string{first, alias}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{first}) {
		t.Fatalf("case-folded queue was not deduplicated: %+v, %v", got, err)
	}
}
