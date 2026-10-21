package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func diskProfileWithRecentViewName(name string) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion":  1,
		"settings":       map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"recentlyViewed": []recentViewEntry{{Type: "item", ID: 7, Name: name}},
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsOverlongRecentViewName(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"ascii", strings.Repeat("a", 161)},
		{"unicode", strings.Repeat("я", 161)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithRecentViewName(tc.value), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted %d-rune name that sanitizer discards", len([]rune(tc.value)))
			}
		})
	}
}

func TestProfileRecoversBackupWhenRecentViewNameTooLong(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"recentlyViewed":[{"type":"monster","id":7,"name":"Saved monster","server":"original"}]}`)
	if err := os.WriteFile(primary, diskProfileWithRecentViewName(strings.Repeat("я", 161)), 0o600); err != nil {
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.RecentlyViewed) != 1 || got.RecentlyViewed[0].Name != "Saved monster" {
		t.Fatalf("valid backup masked by lossy primary: settings=%+v recent=%+v", got.Settings, got.RecentlyViewed)
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

func TestLoadProfileAcceptsRecentViewNameAtUnicodeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	value := strings.Repeat("я", 160)
	if err := os.WriteFile(path, diskProfileWithRecentViewName("  "+value+"  "), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid 160-rune name rejected: %v", err)
	}
	if got := sanitizeProfile(profile).RecentlyViewed; len(got) != 1 || got[0].Name != value {
		t.Fatalf("valid recent view lost: %+v", got)
	}
}

func TestLoadProfileAcceptsLegacyRecentViewWithoutName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"recentlyViewed":[]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile without recent views rejected: %v", err)
	}
}
