package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Profile extensions whose keys exceed the writer's 80-rune limit cannot
// survive sanitizeProfileExtra. Reject such a disk primary so a valid backup
// is not hidden by a profile that silently loses user metadata.
func diskProfileWithExtensionKey(key string) []byte {
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "list"},
		key:             map[string]bool{"important": true},
	})
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsOverlongDiskExtensionKeys(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"ascii", strings.Repeat("a", 81)},
		{"unicode-runes", strings.Repeat("я", 81)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, diskProfileWithExtensionKey(tc.key), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Fatalf("disk profile with %d-rune extension key was accepted", len([]rune(tc.key)))
			}
		})
	}
}

func TestProfileRecoversBackupWhenPrimaryHasOverlongExtensionKey(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"custom":{"preserve":true}}`)
	if err := os.WriteFile(primary, diskProfileWithExtensionKey(strings.Repeat("x", 81)), 0o600); err != nil {
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || string(got.Extra["custom"]) != `{"preserve":true}` {
		t.Fatalf("valid backup hidden by primary with non-persistable key: settings=%+v extra=%v", got.Settings, got.Extra)
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

func TestLoadProfileAcceptsMaximumDiskExtensionKeyLength(t *testing.T) {
	key := strings.Repeat("я", 80)
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, diskProfileWithExtensionKey(key), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid 80-rune extension key rejected: %v", err)
	}
	if string(profile.Extra[key]) != `{"important":true}` {
		t.Fatalf("extension key or value lost: %q", profile.Extra[key])
	}
}
