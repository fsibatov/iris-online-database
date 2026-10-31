package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfileRejectsNullMonsterServer(t *testing.T) {
	for _, tc := range []struct{ key, kind string }{{"server", "monster"}, {"SERVER", "monster"}, {"server", " monster "}} {
		t.Run(tc.key+"/"+tc.kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"` + tc.kind + `","id":7,"name":"Saved monster","` + tc.key + `":null}]}`)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadProfileFile(path); err == nil {
				t.Fatalf("explicit null monster server accepted as empty: %+v", got.RecentlyViewed)
			}
		})
	}
}

func TestProfileRestoresBackupWhenMonsterServerNull(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"monster","id":7,"name":"Primary","server":null}]}`)
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["item:17"],"recentlyViewed":[{"type":"monster","id":17,"name":"Backup","server":"original"}]}`)
	if err := os.WriteFile(primary, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, good, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "item:17" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Server != "original" {
		t.Fatalf("valid backup concealed by null monster server: %+v", got)
	}
	got.Settings.View = "list"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %s, %v", preserved, err)
	}
}

func TestLoadProfileAllowsMissingAndEmptyMonsterServer(t *testing.T) {
	for _, field := range []string{"", `,"server":""`, `,"server":"OR"`} {
		t.Run(field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"monster","id":7,"name":"Saved"` + field + `}]}`)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err != nil {
				t.Fatalf("compatible monster server rejected: %v", err)
			}
		})
	}
}

func TestLoadProfileAllowsLegacyNonMonsterNullServer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Saved","server":null}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("non-monster legacy entry rejected: %v", err)
	}
}
