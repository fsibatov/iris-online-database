package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A profile produced by our writer never contains an extension larger than
// 64 KiB. A malformed primary must not conceal a valid backup whose unknown
// fields would otherwise be silently discarded during sanitization.
func TestLoadProfileRejectsOversizedDiskExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	body := `{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"list"},"custom":"` + strings.Repeat("x", 64<<10) + `"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if profile, err := loadProfileFile(path); err == nil {
		t.Fatalf("non-persistable extension accepted: %d bytes", len(profile.Extra["custom"]))
	}
}

func TestProfileRecoversBackupWhenPrimaryExtensionExceedsLimit(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	bad := `{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"cards"},"custom":"` + strings.Repeat("x", 64<<10) + `"}`
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"list"},"custom":{"keep":true}}`)
	if err := os.WriteFile(primary, []byte(bad), 0o600); err != nil {
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
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || string(got.Extra["custom"]) != `{"keep":true}` {
		t.Fatalf("valid backup concealed by oversized extension: %+v, extra=%s", got.Settings, got.Extra["custom"])
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
		t.Fatalf("valid backup overwritten: %q", preserved)
	}
}

func TestLoadProfileAcceptsMaxSizeDiskExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	value := strings.Repeat("x", (64<<10)-2)
	body, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "original", "theme": "light", "view": "list"},
		"custom":        value,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid boundary extension rejected: %v", err)
	}
	if len(profile.Extra["custom"]) != 64<<10 {
		t.Fatalf("extension changed: %d bytes", len(profile.Extra["custom"]))
	}
}
