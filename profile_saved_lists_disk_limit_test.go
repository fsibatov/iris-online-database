package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The disk loader must not accept lists that the writer silently truncates.
// Otherwise an oversized primary masks the last intact backup.
func diskProfileWithSavedLists(favorites, history int) []byte {
	fav := make([]string, favorites)
	for i := range fav {
		fav[i] = fmt.Sprintf("item:%d", i+1)
	}
	hist := make([]string, history)
	for i := range hist {
		hist[i] = fmt.Sprintf("search-%d", i+1)
	}
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"favorites":     fav,
		"history":       hist,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsUnpersistableSavedListCounts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		favorites int
		history   int
	}{
		{"favorites", 5001, 0},
		{"history", 0, 51},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithSavedLists(tc.favorites, tc.history), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted lossy disk list: favorites=%d history=%d", len(profile.Favorites), len(profile.History))
			}
		})
	}
}

func TestProfileRestoresBackupWhenSavedListsOverflow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		favorites int
		history   int
	}{
		{"favorites", 5001, 0},
		{"history", 0, 51},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			backups := filepath.Join(root, "Backups")
			if err := os.MkdirAll(backups, 0o700); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(root, "profile.json")
			backup := filepath.Join(backups, "profile.json.bak")
			good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["monster:7"],"history":["saved-search"]}`)
			if err := os.WriteFile(primary, diskProfileWithSavedLists(tc.favorites, tc.history), 0o600); err != nil {
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
			if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "monster:7" || len(got.History) != 1 || got.History[0] != "saved-search" {
				t.Fatalf("valid backup hidden by lossy primary: settings=%+v favorites=%d history=%d", got.Settings, len(got.Favorites), len(got.History))
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
				t.Fatalf("valid backup overwritten by lossy primary: %s", preserved)
			}
		})
	}
}

func TestLoadProfileAcceptsSavedListLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, diskProfileWithSavedLists(5000, 50), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid boundary profile rejected: %v", err)
	}
	if len(profile.Favorites) != 5000 || len(profile.History) != 50 {
		t.Fatalf("boundary lists lost: favorites=%d history=%d", len(profile.Favorites), len(profile.History))
	}
}

func TestLoadProfileAcceptsLegacyMissingSavedLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile without saved lists rejected: %v", err)
	}
}
