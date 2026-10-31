package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The writer trims recent-view metadata before enforcing its 240-rune limit.
// A disk value with padding and exactly 240 significant runes is lossless.
func TestLoadProfileAcceptsTrimmedRecentViewMetaAtLimit(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"ascii", strings.Repeat("a", 240)},
		{"unicode", strings.Repeat("я", 240)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			meta := " \t" + tc.value + "\n "
			if err := os.WriteFile(path, diskProfileWithRecentViewMeta(meta), 0o600); err != nil {
				t.Fatal(err)
			}
			profile, err := loadProfileFile(path)
			if err != nil {
				t.Fatalf("lossless metadata rejected: %v", err)
			}
			views := sanitizeProfile(profile).RecentlyViewed
			if len(views) != 1 || views[0].Meta != tc.value {
				t.Fatalf("metadata changed unexpectedly: %+v", views)
			}
		})
	}
}

func TestProfilePrefersValidPrimaryWithTrimmedRecentViewMeta(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	meta := "  " + strings.Repeat("я", 240) + " \t"
	primaryData, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      userSettings{Server: "original", Theme: "light", View: "list"},
		"favorites":     []string{"item:42"},
		"recentlyViewed": []recentViewEntry{
			{Type: "item", ID: 42, Name: "Current item", Meta: meta},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	staleBackup := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"cards"},"favorites":["item:7"]}`)
	if err := os.WriteFile(primary, primaryData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, staleBackup, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "item:42" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Meta != strings.Repeat("я", 240) {
		t.Fatalf("valid primary masked by stale backup: settings=%+v favorites=%+v views=%+v", got.Settings, got.Favorites, got.RecentlyViewed)
	}
	got.Settings.View = "cards"
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(preserved, primaryData) {
		t.Fatalf("last valid primary not preserved in backup: %v", err)
	}
}

func TestLoadProfileRejectsMetaOverLimitAfterTrim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	meta := " \t" + strings.Repeat("я", 241) + " \n"
	if err := os.WriteFile(path, diskProfileWithRecentViewMeta(meta), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err == nil {
		t.Fatal("accepted metadata exceeding 240 significant Unicode characters")
	}
}

func TestLoadProfileAcceptsLegacyRecentViewWithoutMetaAfterTrimCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	body := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[{"type":"item","id":42,"name":"Legacy"}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy recent-view rejected: %v", err)
	}
}
