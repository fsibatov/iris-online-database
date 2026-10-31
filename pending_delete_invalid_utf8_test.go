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

// Go's JSON decoder replaces malformed UTF-8 with U+FFFD. A maintenance
// queue must not silently change a pathname and mask its intact backup.
func pendingDeleteMalformedUTF8(t *testing.T, candidate string, invalid []byte) []byte {
	t.Helper()
	const marker = "INVALID_UTF8_MARKER"
	data, err := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(candidate, marker+".bin")}})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Replace(data, []byte(marker), invalid, 1)
}

func TestPendingDeleteRejectsMalformedUTF8Paths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bytes []byte
	}{
		{"invalid-byte", []byte{0xff}},
		{"truncated-sequence", []byte{0xc3}},
		{"bad-continuation", []byte{0xe2, 0x28, 0xa1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			queue := filepath.Join(root, "pending-delete.json")
			if err := os.WriteFile(queue, pendingDeleteMalformedUTF8(t, filepath.Join(root, "Cache"), tc.bytes), 0o600); err != nil {
				t.Fatal(err)
			}
			if list, err := loadPendingDeleteList(queue); err == nil {
				t.Fatalf("malformed UTF-8 accepted as paths: %+v", list.Paths)
			}
		})
	}
}

func TestPendingDeleteMalformedUTF8RecoversBackup(t *testing.T) {
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
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(queue, pendingDeleteMalformedUTF8(t, cache, []byte{0xff}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(queue, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup was not replayed: %v", err)
	}
}

func TestAppendPendingDeleteMalformedUTF8PreservesBackup(t *testing.T) {
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
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(queue, pendingDeleteMalformedUTF8(t, cache, []byte{0xff}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(queue, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	list, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(list.Paths, []string{old, next}) {
		t.Fatalf("valid backup entries lost: %+v, %v", list.Paths, err)
	}
	preserved, err := os.ReadFile(queue + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}
