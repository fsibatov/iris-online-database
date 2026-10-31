package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Settings is a fixed-shape disk object. Unknown nested keys cannot survive
// a profile rewrite and must not conceal a complete backup.
func TestLoadProfileRejectsUnknownDiskSettingsFields(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"extra-preference", `{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list","language":"ru"}}`},
		{"extra-nested-object", `{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list","custom":{"keep":true}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Fatal("accepted settings field that cannot survive a disk rewrite")
			}
		})
	}
}

func TestProfileRecoversBackupWhenPrimaryHasUnknownSettings(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list","futureSetting":true}}`)
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"custom":{"keep":true}}`)
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || got.Settings.View != "cards" || string(got.Extra["custom"]) != `{"keep":true}` {
		t.Fatalf("valid backup was hidden: settings=%+v, extra=%v", got.Settings, got.Extra)
	}
	got.Settings.Theme = "dark"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(good) {
		t.Fatalf("valid backup overwritten by lossy primary: %q", preserved)
	}
}

func TestLoadProfileAllowsCanonicalAndCaseAliasedDiskSettings(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`,
		`{"SchemaVersion":1,"Settings":{"SERVER":"original","Theme":"light","VIEW":"cards"}}`,
		`{"schemaVersion":1,"favorites":[]}`, // Legacy profiles without settings remain readable.
	} {
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProfileFile(path); err != nil {
			t.Fatalf("compatible profile rejected: %v", err)
		}
	}
}
