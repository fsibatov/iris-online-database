package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// History entries longer than 120 Unicode characters cannot survive the
// profile sanitizer. Reject a lossy primary instead of masking a valid backup.
func diskProfileWithHistoryEntry(value string) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		"history":       []string{value},
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsOverlongHistoryEntry(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"ascii", strings.Repeat("a", 121)},
		{"unicode", strings.Repeat("я", 121)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithHistoryEntry(tc.value), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Fatalf("accepted history entry with %d runes; sanitizer discards it", len([]rune(tc.value)))
			}
		})
	}
}

func TestProfileRecoversBackupWhenHistoryEntryTooLong(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"history":["important saved query"]}`)
	if err := os.WriteFile(primary, diskProfileWithHistoryEntry(strings.Repeat("я", 121)), 0o600); err != nil {
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || len(got.History) != 1 || got.History[0] != "important saved query" {
		t.Fatalf("valid backup masked by lossy primary: settings=%+v history=%q", got.Settings, got.History)
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

func TestLoadProfileAcceptsHistoryEntryAtUnicodeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	value := strings.Repeat("я", 120)
	if err := os.WriteFile(path, diskProfileWithHistoryEntry("  "+value+"  "), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid 120-rune history entry rejected: %v", err)
	}
	if got := sanitizeProfile(profile).History; len(got) != 1 || got[0] != value {
		t.Fatalf("valid history entry lost: %q", got)
	}
}

func TestLoadProfileAcceptsLegacyMissingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy profile without history rejected: %v", err)
	}
}
