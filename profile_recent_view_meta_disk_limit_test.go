package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A disk profile with overlong recent-view metadata cannot round-trip through
// sanitizeRecentViews. Reject it instead of hiding a valid backup and losing text.
func diskProfileWithRecentViewMeta(meta string) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion":  1,
		"settings":       map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"recentlyViewed": []recentViewEntry{{Type: "item", ID: 7, Name: "Saved item", Meta: meta}},
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsOverlongRecentViewMeta(t *testing.T) {
	for _, tc := range []struct{ name, meta string }{
		{"ascii", strings.Repeat("a", 241)},
		{"unicode", strings.Repeat("я", 241)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithRecentViewMeta(tc.meta), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted %d-rune metadata; writer truncates to 240", len([]rune(tc.meta)))
			}
		})
	}
}

func TestProfileRecoversBackupWhenRecentViewMetaTooLong(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"recentlyViewed":[{"type":"monster","id":7,"name":"Backup monster","meta":"keep this","server":"original"}]}`)
	if err := os.WriteFile(primary, diskProfileWithRecentViewMeta(strings.Repeat("z", 241)), 0o600); err != nil {
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Meta != "keep this" {
		t.Fatalf("valid backup hidden by lossy primary: settings=%+v recent=%+v", got.Settings, got.RecentlyViewed)
	}
	got.Settings.View = "list"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(good) {
		t.Fatalf("valid backup overwritten by invalid primary: %q", preserved)
	}
}

func TestLoadProfileAcceptsRecentViewMetaAtLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	meta := strings.Repeat("я", 240)
	if err := os.WriteFile(path, diskProfileWithRecentViewMeta(meta), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid 240-rune metadata rejected: %v", err)
	}
	if len(profile.RecentlyViewed) != 1 || profile.RecentlyViewed[0].Meta != meta {
		t.Fatalf("valid recent-view metadata lost: %+v", profile.RecentlyViewed)
	}
}

func TestLoadProfileAcceptsLegacyRecentViewWithoutMeta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":1,"name":"Legacy"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil || len(profile.RecentlyViewed) != 1 || profile.RecentlyViewed[0].Name != "Legacy" {
		t.Fatalf("legacy profile rejected: profile=%+v err=%v", profile.RecentlyViewed, err)
	}
}
