package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Unknown fields in disk recent-view entries cannot round-trip through the
// fixed-shape recentViewEntry writer. A lossy primary must not hide a backup.
func TestLoadProfileRejectsUnknownRecentViewFields(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"future-source", `{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Saved item","source":"future-client"}]}`},
		{"future-nested", `{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Saved item","future":{"keep":true}}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted disk recent-view fields that sanitizeProfile cannot preserve: %+v", profile.RecentlyViewed)
			}
		})
	}
}

func TestProfileRecoversBackupWhenRecentViewContainsUnknownFields(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Primary","source":"future-client"}]}`)
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"recentlyViewed":[{"type":"monster","id":7,"name":"Backup monster","server":"original"}]}`)
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Name != "Backup monster" {
		t.Fatalf("valid backup masked by lossy primary: settings=%+v views=%+v", got.Settings, got.RecentlyViewed)
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
		t.Fatalf("backup overwritten by invalid primary: %s", preserved)
	}
}

func TestLoadProfileAllowsKnownCaseAliasedRecentViewFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"RecentlyViewed":[{"TYPE":"monster","ID":7,"NAME":"Known","META":"note","SERVER":"original"}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadProfileFile(path)
	if err != nil || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Name != "Known" {
		t.Fatalf("compatible case-aliased recent view rejected: %+v, %v", got.RecentlyViewed, err)
	}
}

func TestLoadProfileAllowsLegacyMissingRecentViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile rejected: %v", err)
	}
}
