package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Persisted profiles must have the same duplicate-key protection as the API.
// Otherwise a malformed primary file silently overwrites a preference and
// prevents newProfileStore from recovering the valid backup.
func TestLoadProfileRejectsDuplicateJSONKeys(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"top-level", `{"schemaVersion":1,"settings":{"theme":"dark"},"settings":{"theme":"light"}}`},
		{"nested-settings", `{"schemaVersion":1,"settings":{"theme":"dark","theme":"light"}}`},
		{"escaped-nested-key", `{"schemaVersion":1,"settings":{"theme":"dark","the\u006de":"light"}}`},
		{"extra-metadata", `{"schemaVersion":1,"customFlag":false,"customFlag":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Errorf("persisted profile with duplicate JSON keys accepted: %+v", profile.Settings)
			}
		})
	}
}

func TestProfileStoreFallsBackFromDuplicatePrimaryToValidBackup(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	if err := os.WriteFile(primary, []byte(`{"schemaVersion":1,"settings":{"theme":"dark","theme":"light"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"dark","view":"cards"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.Settings.Theme != "dark" || got.Settings.Server != "original" || got.Settings.View != "cards" {
		t.Fatalf("valid backup was not recovered: %+v", got.Settings)
	}
}

func TestLoadProfileAllowsValidSingleAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"SchemaVersion":1,"Settings":{"Theme":"light","SERVER":"original","VIEW":"list"},"customFlag":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings.Theme != "light" || got.Settings.Server != "original" || got.Settings.View != "list" || got.Extra["customFlag"] == nil {
		t.Fatalf("valid aliases or metadata were lost: %+v", got)
	}
}
