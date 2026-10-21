package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The writer retains at most 64 unknown profile fields. An oversized primary
// must not mask a readable backup and silently discard user metadata.
func profileWithExtensionCount(count int) []byte {
	fields := map[string]any{
		"schemaVersion": 1,
		"settings":      map[string]string{"server": "kiss", "theme": "dark", "view": "cards"},
	}
	for i := 0; i < count; i++ {
		fields[fmt.Sprintf("custom%03d", i)] = i
	}
	data, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return data
}

func TestLoadProfileRejectsTooManyDiskExtensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, profileWithExtensionCount(65), 0o600); err != nil {
		t.Fatal(err)
	}
	if profile, err := loadProfileFile(path); err == nil {
		t.Fatalf("non-persistable profile accepted with %d extension fields", len(profile.Extra))
	}
}

func TestProfileRecoversBackupWhenPrimaryHasTooManyExtensions(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	good := []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"list"},"custom":{"keep":true}}`)
	if err := os.WriteFile(primary, profileWithExtensionCount(65), 0o600); err != nil {
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
		t.Fatalf("valid backup hidden by primary with too many fields: %+v, extra=%v", got.Settings, got.Extra)
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
		t.Fatalf("valid backup overwritten by invalid primary: %q", preserved)
	}
}

func TestLoadProfileAcceptsMaximumDiskExtensionCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, profileWithExtensionCount(64), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		t.Fatalf("valid 64-field profile rejected: %v", err)
	}
	if len(profile.Extra) != 64 {
		t.Fatalf("extension count changed: %d", len(profile.Extra))
	}
}
