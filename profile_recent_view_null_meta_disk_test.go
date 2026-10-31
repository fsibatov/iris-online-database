package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfileRejectsNullRecentViewMeta(t *testing.T) {
	for _, tc := range []struct{ key, kind string }{{"meta", "item"}, {"META", "monster"}, {"meta", "recipe"}} {
		t.Run(tc.key+"/"+tc.kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"` + tc.kind + `","id":7,"name":"Saved","` + tc.key + `":null}]}`)
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadProfileFile(path); err == nil {
				t.Fatalf("explicit null metadata accepted as empty: %+v", got.RecentlyViewed)
			}
		})
	}
}

func TestProfileRestoresBackupWhenRecentViewMetaNull(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Primary","meta":null}]}`)
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["item:17"],"recentlyViewed":[{"type":"item","id":17,"name":"Backup","meta":"Important details"}]}`)
	if err := os.WriteFile(primary, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, good, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "item:17" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Meta != "Important details" {
		t.Fatalf("valid backup masked by null metadata: %+v", got)
	}
	got.Settings.View = "list"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestLoadProfileAcceptsMissingAndEmptyRecentViewMeta(t *testing.T) {
	for _, field := range []string{"", `,"meta":""`, `,"META":" note "`} {
		t.Run(field, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Saved"` + field + `}]}`)
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := loadProfileFile(path)
			if err != nil || len(got.RecentlyViewed) != 1 {
				t.Fatalf("valid metadata rejected: %+v, %v", got.RecentlyViewed, err)
			}
		})
	}
}

func TestLoadProfileAllowsLegacyNonMonsterNullServerWithMeta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Saved","server":null,"meta":"note"}]}`)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy server rejected: %v", err)
	}
}
