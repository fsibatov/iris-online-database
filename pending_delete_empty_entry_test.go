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

// encoding/json decodes a null array element into the empty string when
// unmarshalling []string. An unusable primary must not mask a valid backup.
func TestPendingDeleteListRejectsBlankPathEntries(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"null", `{"paths":[null]}`},
		{"empty", `{"paths":[""]}`},
		{"whitespace", `{"paths":[" \t "]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pending-delete.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(path); err == nil {
				t.Fatalf("unusable queue entry accepted: %+v", got)
			}
		})
	}
}

func TestPendingDeletesRecoverBackupFromBlankPathEntries(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"null", `{"paths":[null]}`},
		{"empty", `{"paths":[""]}`},
		{"mixed", `{"paths":["valid-but-not-absolute",null]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cache := filepath.Join(root, "Cache")
			if err := os.Mkdir(cache, 0o700); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(cache, "obsolete.bin")
			if err := os.WriteFile(victim, []byte("obsolete"), 0o600); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(root, "pending-delete.json")
			if err := os.WriteFile(primary, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			backup, err := json.Marshal(pendingDeleteList{Paths: []string{victim}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
				t.Fatal(err)
			}
			processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
			if _, err := os.Stat(victim); !os.IsNotExist(err) {
				t.Fatalf("valid backup was ignored; obsolete file remains: %v", err)
			}
		})
	}
}

func TestAppendPendingDeletesRecoversBackupFromBlankEntry(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
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
	primary := filepath.Join(root, "pending-delete.json")
	if err := os.WriteFile(primary, []byte(`{"paths":[null]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(primary, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	list, err := loadPendingDeleteList(primary)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list.Paths, []string{old, next}) {
		t.Errorf("queue = %q; want recovered and appended paths", list.Paths)
	}
	preserved, err := os.ReadFile(primary + ".bak")
	if err != nil || !reflect.DeepEqual(preserved, backup) {
		t.Errorf("valid backup overwritten: %s, %v", preserved, err)
	}
}

func TestPendingDeleteListAcceptsNonemptyPaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pending-delete.json")
	absolute := filepath.Join(root, "Cache", "old.bin")
	data, err := json.Marshal(pendingDeleteList{Paths: []string{absolute}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := loadPendingDeleteList(path)
	if err != nil || len(list.Paths) != 1 || list.Paths[0] != absolute {
		t.Fatalf("valid path rejected: %+v, %v", list, err)
	}
}
