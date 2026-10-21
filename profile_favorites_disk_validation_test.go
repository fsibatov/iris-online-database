package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func diskProfileWithFavorites(entries ...string) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"favorites":     entries,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsUnpersistableFavoriteKeys(t *testing.T) {
	for _, tc := range []struct{ name, favorite string }{
		{"future-category", "recipe:7"},
		{"invalid-id", "item:abc"},
		{"overlong-id", "monster:123456789012345678901"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithFavorites(tc.favorite), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted favorite %q which writer discards", tc.favorite)
			}
		})
	}
}

func TestProfileRestoresBackupWhenFavoriteCannotPersist(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["monster:7"],"history":["saved-search"]}`)
	if err := os.WriteFile(primary, diskProfileWithFavorites("recipe:7"), 0o600); err != nil {
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "monster:7" || len(got.History) != 1 {
		t.Fatalf("valid backup masked by lossy primary: settings=%+v favorites=%+v history=%+v", got.Settings, got.Favorites, got.History)
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
		t.Fatalf("valid backup overwritten by invalid primary: %s", preserved)
	}
}

func TestLoadProfileAcceptsValidFavoriteKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := diskProfileWithFavorites("item:99999999999999999999", "monster:7", "title:2", "transformation:3")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := sanitizeProfile(profile).Favorites; len(got) != 4 {
		t.Fatalf("valid favorites lost: %+v", got)
	}
}

func TestLoadProfileAcceptsLegacyWithoutFavorites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile rejected: %v", err)
	}
}
