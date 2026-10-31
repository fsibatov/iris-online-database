package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// encoding/json treats differently cased recent-view field names as the same
// struct member. The disk loader must not accept both and silently discard one.
func TestLoadProfileRejectsConflictingRecentViewFieldAliases(t *testing.T) {
	cases := []struct{ name, entry string }{
		{"name", `{"type":"item","id":7,"name":"First","NAME":"Second"}`},
		{"type", `{"type":"item","TYPE":"monster","id":7,"name":"Saved","server":"original"}`},
		{"id", `{"type":"item","id":7,"ID":8,"name":"Saved"}`},
		{"meta", `{"type":"item","id":7,"name":"Saved","meta":"First","META":"Second"}`},
		{"server", `{"type":"monster","id":7,"name":"Saved","server":"kiss","SERVER":"original"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[` + tc.entry + `]}`)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadProfileFile(path); err == nil {
				t.Fatalf("conflicting case aliases accepted and one value lost: %+v", got.RecentlyViewed)
			}
		})
	}
}

func TestProfileRestoresBackupWhenRecentViewFieldsConflict(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Primary","NAME":"Override"}]}`)
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["item:17"],"recentlyViewed":[{"type":"item","id":17,"name":"Backup item"}]}`)
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "item:17" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Name != "Backup item" {
		t.Fatalf("conflicting primary concealed valid backup: %+v", got)
	}
	got.Settings.View = "list"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %s", preserved)
	}
}

func TestLoadProfileAcceptsSingleCaseAliasedRecentViewFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"RecentlyViewed":[{"TYPE":"monster","ID":7,"NAME":"Saved","META":"note","SERVER":"OR"}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	views := sanitizeProfile(got).RecentlyViewed
	if len(views) != 1 || views[0].Type != "monster" || views[0].Server != "original" || views[0].Meta != "note" {
		t.Fatalf("valid single aliases changed: %+v", views)
	}
}

func TestLoadProfileAcceptsLegacyMissingRecentViewFieldAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile rejected: %v", err)
	}
}
