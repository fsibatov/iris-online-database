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

// A syntactically valid JSON document can still be an invalid maintenance
// queue. Rejecting it lets the caller recover the last good backup.
func TestPendingDeleteListRejectsInvalidPathsEnvelope(t *testing.T) {
	cases := []struct{ name, body string }{
		{"root-null", `null`},
		{"missing-paths", `{}`},
		{"paths-null", `{"paths":null}`},
		{"alias-null", `{"Paths":null}`},
		{"conflicting-aliases", `{"paths":["first"],"Paths":["second"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pending-delete.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(path); err == nil {
				t.Fatalf("invalid maintenance queue accepted: %+v", got)
			}
		})
	}
}

func TestPendingDeletesRecoverBackupFromInvalidPathsEnvelope(t *testing.T) {
	cases := []struct{ name, body string }{
		{"root-null", `null`},
		{"missing-paths", `{}`},
		{"paths-null", `{"paths":null}`},
		{"conflicting-aliases", `{"paths":[],"Paths":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			cache := filepath.Join(base, "Cache")
			if err := os.Mkdir(cache, 0o700); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(cache, "obsolete.bin")
			outside := filepath.Join(base, "preserve.bin")
			for _, path := range []string{victim, outside} {
				if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			primary := filepath.Join(base, "pending-delete.json")
			if err := os.WriteFile(primary, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			backup, err := json.Marshal(pendingDeleteList{Paths: []string{victim, outside}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(primary+".bak", backup, 0o600); err != nil {
				t.Fatal(err)
			}
			processPendingDeletes(primary, []string{cache}, "", log.New(io.Discard, "", 0))
			if _, err := os.Stat(victim); !os.IsNotExist(err) {
				t.Errorf("backup queue ignored, obsolete file remains: %v", err)
			}
			if _, err := os.Stat(outside); err != nil {
				t.Errorf("out-of-root file was changed: %v", err)
			}
		})
	}
}

func TestAppendPendingDeletesRecoversBackupFromNullPaths(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary := filepath.Join(base, "pending-delete.json")
	if err := os.WriteFile(primary, []byte(`{"paths":null}`), 0o600); err != nil {
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
		t.Errorf("pending queue = %v; want original and new paths", list.Paths)
	}
	gotBackup, err := os.ReadFile(primary + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotBackup, backup) {
		t.Errorf("valid backup was overwritten by invalid primary")
	}
}

func TestPendingDeleteListAcceptsValidEmptyAndLegacyAliases(t *testing.T) {
	for _, body := range []string{`{"paths":[]}`, `{"Paths":["test"]}`} {
		path := filepath.Join(t.TempDir(), "pending-delete.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadPendingDeleteList(path); err != nil {
			t.Errorf("valid maintenance queue %s rejected: %v", body, err)
		}
	}
}
