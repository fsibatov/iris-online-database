package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func diskProfileWithRecentViewEntries(entries ...recentViewEntry) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion":  1,
		"settings":       map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"recentlyViewed": entries,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsDuplicateRecentViewIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []recentViewEntry
	}{
		{"item-duplicate", []recentViewEntry{
			{Type: "item", ID: 42, Name: "First"},
			{Type: "item", ID: 42, Name: "Second", Server: "legacy"},
		}},
		{"monster-server-alias", []recentViewEntry{
			{Type: "monster", ID: 42, Name: "First", Server: "original"},
			{Type: "monster", ID: 42, Name: "Second", Server: " OR "},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithRecentViewEntries(tc.entries...), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted disk entries silently deduplicated by writer: %+v", sanitizeProfile(profile).RecentlyViewed)
			}
		})
	}
}

func TestProfileRecoversBackupWhenRecentViewsDuplicate(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := diskProfileWithRecentViewEntries(
		recentViewEntry{Type: "item", ID: 7, Name: "Primary first"},
		recentViewEntry{Type: "item", ID: 7, Name: "Primary second"},
	)
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"recentlyViewed":[{"type":"monster","id":9,"name":"Backup monster","server":"original"}]}`)
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
		t.Fatalf("duplicate primary masked valid backup: settings=%+v views=%+v", got.Settings, got.RecentlyViewed)
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

func TestLoadProfileAcceptsDistinctRecentViewIdentities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := diskProfileWithRecentViewEntries(
		recentViewEntry{Type: "monster", ID: 7, Name: "Kiss", Server: "kiss"},
		recentViewEntry{Type: "monster", ID: 7, Name: "Original", Server: "original"},
		recentViewEntry{Type: "item", ID: 7, Name: "Item"},
		recentViewEntry{Type: "item", ID: 8, Name: "Another item", Server: "legacy"},
	)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if views := sanitizeProfile(profile).RecentlyViewed; len(views) != 4 {
		t.Fatalf("distinct identities lost: %+v", views)
	}
}

func TestLoadProfileAllowsLegacyWithoutRecentViewList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile rejected: %v", err)
	}
}
