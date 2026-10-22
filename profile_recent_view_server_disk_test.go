package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfileRejectsUnpersistableMonsterServer(t *testing.T) {
	for _, server := range []string{"unknown", "original-future", "kiss/original"} {
		t.Run(server, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"monster","id":7,"name":"Saved monster","server":"` + server + `"}]}`)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted monster server discarded by sanitizer: %+v", profile.RecentlyViewed)
			}
		})
	}
}

func TestProfileRestoresBackupWhenMonsterServerInvalid(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"monster","id":7,"name":"Primary monster","server":"future"}]}`)
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
		t.Fatalf("valid backup overwritten: %s", preserved)
	}
}

func TestLoadProfileAcceptsSupportedMonsterServerAliases(t *testing.T) {
	for _, tc := range []struct{ server, expected string }{
		{"kiss", "kiss"}, {"original", "original"}, {" OR ", "original"}, {"", ""},
	} {
		t.Run(tc.server, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"monster","id":7,"name":"Saved monster","server":"` + tc.server + `"}]}`)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			profile, err := loadProfileFile(path)
			if err != nil {
				t.Fatal(err)
			}
			views := sanitizeProfile(profile).RecentlyViewed
			if len(views) != 1 || views[0].Server != tc.expected {
				t.Fatalf("server=%q views=%+v", tc.server, views)
			}
		})
	}
}

func TestLoadProfileAllowsLegacyRecentViewServerRules(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":7,"name":"Saved item","server":"old-client"}]}`),
		[]byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`),
	} {
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProfileFile(path); err != nil {
			t.Fatalf("legacy profile rejected: %v", err)
		}
	}
}
