package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A partially written settings object must not conceal a complete backup.
// Old profiles without the settings object remain readable for compatibility.
func TestLoadProfileRejectsPartialSettingsOnDisk(t *testing.T) {
	cases := []struct{ name, body string }{
		{"missing-server", `{"schemaVersion":1,"settings":{"theme":"dark","view":"list"}}`},
		{"missing-theme", `{"schemaVersion":1,"settings":{"server":"original","view":"cards"}}`},
		{"missing-view", `{"schemaVersion":1,"settings":{"server":"original","theme":"light"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Errorf("incomplete on-disk settings accepted: %+v", profile.Settings)
			}
		})
	}
}

func TestProfileStoreRecoversCompleteBackupAfterPartialSettings(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	incomplete := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark"}}`)
	original := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"}}`)
	if err := os.WriteFile(primary, incomplete, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get().Settings
	if got.Server != "original" || got.Theme != "light" || got.View != "cards" {
		t.Fatalf("complete backup hidden by partial primary: %+v", got)
	}
	next := store.Get()
	next.Settings.Theme = "dark"
	if err := store.Replace(next); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(original) {
		t.Fatalf("valid backup replaced by incomplete primary: %q", preserved)
	}
}

func TestLoadProfilePartialSettingsGuardPreservesLegacy(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"favorites":[],"history":[]}`,
		`{"SchemaVersion":1,"Settings":{"SERVER":"original","Theme":"light","VIEW":"cards"},"custom":true}`,
	} {
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProfileFile(path); err != nil {
			t.Fatalf("compatible profile rejected: %s: %v", body, err)
		}
	}
}
