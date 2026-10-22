package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// encoding/json decodes null elements of []string as empty strings. The
// history writer also discards empty and whitespace-only entries. A disk
// primary with such entries must not conceal the last intact backup.
func diskProfileWithNullListElement(field string) []byte {
	lists := map[string]any{
		"favorites":      []any{"item:7", nil},
		"history":        []any{"search", nil},
		"recentlyViewed": []any{map[string]any{"type": "item", "id": 7, "name": "Saved item"}, nil},
	}
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		field:           lists[field],
	})
	if err != nil {
		panic(err)
	}
	return data
}

func diskProfileWithInvalidHistoryEntry(value any) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"history":       []any{"search", value},
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsNullAndBlankListElementsOnDisk(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"favorites-null", diskProfileWithNullListElement("favorites")},
		{"history-null", diskProfileWithNullListElement("history")},
		{"history-empty", diskProfileWithInvalidHistoryEntry("")},
		{"history-whitespace", diskProfileWithInvalidHistoryEntry(" \t ")},
		{"history-unicode-whitespace", diskProfileWithInvalidHistoryEntry(" \u2003 ")},
		{"recentlyViewed-null", diskProfileWithNullListElement("recentlyViewed")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, tc.body, 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted lossy list element: favorites=%v history=%v recent=%v", profile.Favorites, profile.History, profile.RecentlyViewed)
			}
		})
	}
}

func TestProfileRestoresBackupWhenDiskListHasNullOrBlankElement(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"favorites-null", diskProfileWithNullListElement("favorites")},
		{"history-null", diskProfileWithNullListElement("history")},
		{"history-empty", diskProfileWithInvalidHistoryEntry("")},
		{"history-whitespace", diskProfileWithInvalidHistoryEntry(" \t ")},
		{"history-unicode-whitespace", diskProfileWithInvalidHistoryEntry(" \u2003 ")},
		{"recentlyViewed-null", diskProfileWithNullListElement("recentlyViewed")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			backups := filepath.Join(root, "Backups")
			if err := os.MkdirAll(backups, 0o700); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(root, "profile.json")
			backup := filepath.Join(backups, "profile.json.bak")
			good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"favorites":["item:17"],"history":["backup-search"],"recentlyViewed":[{"type":"item","id":17,"name":"Backup item"}]}`)
			if err := os.WriteFile(primary, tc.body, 0o600); err != nil {
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
			if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.Favorites) != 1 || got.Favorites[0] != "item:17" || len(got.History) != 1 || got.History[0] != "backup-search" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Name != "Backup item" {
				t.Fatalf("lossy primary concealed intact backup: %+v", got)
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
				t.Fatalf("intact backup overwritten: %s", preserved)
			}
		})
	}
}

func TestLoadProfileAcceptsLegacyNullAndMissingLists(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"favorites":null,"history":null,"recentlyViewed":null}`,
		`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`,
	} {
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProfileFile(path); err != nil {
			t.Fatalf("compatible legacy lists rejected: %v", err)
		}
	}
}

func TestLoadProfileAcceptsNonNullListElements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"favorites":["item:7"],"history":["search"],"recentlyViewed":[{"type":"item","id":7,"name":"Saved item"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clean := sanitizeProfile(profile)
	if len(clean.Favorites) != 1 || len(clean.History) != 1 || len(clean.RecentlyViewed) != 1 {
		t.Fatalf("valid lists lost: favorites=%v history=%v recent=%v", clean.Favorites, clean.History, clean.RecentlyViewed)
	}
}
