package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func diskProfileWithRecentViewIdentity(entry recentViewEntry) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion":  1,
		"settings":       map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"recentlyViewed": []recentViewEntry{entry},
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsUnpersistableRecentViewIdentity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry recentViewEntry
	}{
		{"unknown-type", recentViewEntry{Type: "future", ID: 7, Name: "Saved"}},
		{"zero-id", recentViewEntry{Type: "item", ID: 0, Name: "Saved"}},
		{"negative-id", recentViewEntry{Type: "monster", ID: -1, Name: "Saved", Server: "original"}},
		{"empty-name", recentViewEntry{Type: "item", ID: 7, Name: "   "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithRecentViewIdentity(tc.entry), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted recent view discarded by writer: %+v", profile.RecentlyViewed)
			}
		})
	}
}

func TestProfileRecoversBackupWhenRecentViewIdentityInvalid(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"recentlyViewed":[{"type":"monster","id":7,"name":"Backup monster","server":"original"}]}`)
	if err := os.WriteFile(primary, diskProfileWithRecentViewIdentity(recentViewEntry{Type: "future", ID: 7, Name: "Primary"}), 0o600); err != nil {
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

func TestLoadProfileAcceptsSupportedRecentViewIdentities(t *testing.T) {
	for _, entry := range []recentViewEntry{
		{Type: "item", ID: 7, Name: "Item"},
		{Type: "recipe", ID: 8, Name: "Recipe"},
		{Type: "monster", ID: 9, Name: "Monster", Server: "original"},
		{Type: "title", ID: 10, Name: "Title"},
		{Type: "transformation", ID: 11, Name: "Transformation"},
	} {
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, diskProfileWithRecentViewIdentity(entry), 0o600); err != nil {
			t.Fatal(err)
		}
		profile, err := loadProfileFile(path)
		if err != nil {
			t.Fatalf("valid recent view rejected: %+v: %v", entry, err)
		}
		if got := sanitizeProfile(profile).RecentlyViewed; len(got) != 1 || got[0].ID != entry.ID {
			t.Fatalf("valid recent view lost: %+v", got)
		}
	}
}

func TestLoadProfileAcceptsLegacyWithoutRecentViewIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile rejected: %v", err)
	}
}
