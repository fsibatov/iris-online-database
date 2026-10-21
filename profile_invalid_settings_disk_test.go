package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The disk writer always stores valid preferences. A damaged primary must
// not conceal a complete backup and silently reset user-selected settings.
func TestLoadProfileRejectsInvalidSettingValuesOnDisk(t *testing.T) {
	cases := []struct{ name, settings string }{
		{"server", `{"server":"unknown","theme":"light","view":"cards"}`},
		{"theme", `{"server":"original","theme":"unknown","view":"cards"}`},
		{"view", `{"server":"original","theme":"light","view":"unknown"}`},
		{"empty-server", `{"server":"","theme":"light","view":"cards"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"settings":`+tc.settings+`}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Errorf("invalid disk setting accepted: %+v", profile.Settings)
			}
		})
	}
}

func TestProfileStoreRecoversBackupAfterInvalidSetting(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	invalid := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"invalid","view":"cards"}}`)
	valid := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"list"}}`)
	if err := os.WriteFile(primary, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get().Settings
	if got.Server != "original" || got.Theme != "light" || got.View != "list" {
		t.Fatalf("valid backup hidden by invalid primary: %+v", got)
	}
	next := store.Get()
	next.Settings.View = "cards"
	if err := store.Replace(next); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(valid) {
		t.Fatalf("valid backup overwritten by invalid primary: %q", preserved)
	}
}

func TestLoadProfileValidSettingsAndLegacyWithoutSettings(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"favorites":[],"history":[]}`,
		`{"SchemaVersion":1,"Settings":{"SERVER":"original","Theme":"light","VIEW":"cards"}}`,
		`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`,
	} {
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProfileFile(path); err != nil {
			t.Fatalf("valid profile rejected: %s: %v", body, err)
		}
	}
}
