package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func diskProfileWithDuplicateSavedLists(favorites, history []string) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"favorites":     favorites,
		"history":       history,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsDuplicateSavedListEntries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		favorites []string
		history   []string
	}{
		{"duplicate-favorites", []string{"item:7", "item:7"}, nil},
		{"trimmed-favorite-alias", []string{"item:7", " item:7 "}, nil},
		{"duplicate-history", nil, []string{"search", "search"}},
		{"trimmed-history-alias", nil, []string{"search", " search "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithDuplicateSavedLists(tc.favorites, tc.history), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted lossy disk list: favorites=%v history=%v", sanitizeProfile(profile).Favorites, sanitizeProfile(profile).History)
			}
		})
	}
}

func TestProfileRecoversBackupWhenSavedListsContainDuplicates(t *testing.T) {
	for _, tc := range []struct {
		name      string
		favorites []string
		history   []string
	}{
		{"favorites", []string{"monster:9", " monster:9 "}, nil},
		{"history", nil, []string{"search", " search "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			backups := filepath.Join(root, "Backups")
			if err := os.MkdirAll(backups, 0o700); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(root, "profile.json")
			backup := filepath.Join(backups, "profile.json.bak")
			good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["item:17"],"history":["backup-search"]}`)
			if err := os.WriteFile(primary, diskProfileWithDuplicateSavedLists(tc.favorites, tc.history), 0o600); err != nil {
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
			if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "item:17" || len(got.History) != 1 || got.History[0] != "backup-search" {
				t.Fatalf("duplicate primary concealed valid backup: %+v", got)
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
				t.Fatalf("backup overwritten: %s", preserved)
			}
		})
	}
}

func TestLoadProfileAcceptsDistinctSavedListEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, diskProfileWithDuplicateSavedLists([]string{"item:7", "monster:7"}, []string{"search", "another-search"}), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clean := sanitizeProfile(profile)
	if len(clean.Favorites) != 2 || len(clean.History) != 2 {
		t.Fatalf("distinct entries lost: favorites=%v history=%v", clean.Favorites, clean.History)
	}
}

func TestLoadProfileAcceptsLegacyWithoutSavedLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile rejected: %v", err)
	}
}
