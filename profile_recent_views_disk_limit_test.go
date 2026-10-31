package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A disk profile with more recent views than the writer retains must not
// hide a valid backup and silently discard part of the browsing history.
func diskProfileWithRecentViews(count int) []byte {
	views := make([]recentViewEntry, 0, count)
	for i := 1; i <= count; i++ {
		views = append(views, recentViewEntry{Type: "item", ID: i, Name: fmt.Sprintf("Item %d", i)})
	}
	data, err := json.Marshal(map[string]any{
		"schemaVersion":  1,
		"settings":       map[string]string{"server": "kiss", "theme": "dark", "view": "cards"},
		"recentlyViewed": views,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsUnpersistableRecentViewCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, diskProfileWithRecentViews(9), 0o600); err != nil {
		t.Fatal(err)
	}
	if profile, err := loadProfileFile(path); err == nil {
		t.Fatalf("accepted %d recent views; writer keeps only 8", len(profile.RecentlyViewed))
	}
}

func TestProfileRestoresBackupWhenPrimaryHasTooManyRecentViews(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"list"},"recentlyViewed":[{"type":"monster","id":7,"name":"Backup monster","server":"original"}]}`)
	if err := os.WriteFile(primary, diskProfileWithRecentViews(9), 0o600); err != nil {
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
		t.Fatalf("valid backup hidden by lossy primary: settings=%+v recent=%+v", got.Settings, got.RecentlyViewed)
	}
	got.Settings.View = "cards"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(good) {
		t.Fatalf("valid backup overwritten by unpersistable primary: %q", preserved)
	}
}

func TestLoadProfileAcceptsEightRecentViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, diskProfileWithRecentViews(8), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid eight-view profile rejected: %v", err)
	}
	if len(profile.RecentlyViewed) != 8 {
		t.Fatalf("recent views lost: %d", len(profile.RecentlyViewed))
	}
}

func TestLoadProfileAcceptsLegacyMissingRecentViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("legacy profile without recent views rejected: %v", err)
	}
	if len(profile.RecentlyViewed) != 0 {
		t.Fatalf("unexpected legacy recent views: %+v", profile.RecentlyViewed)
	}
}
