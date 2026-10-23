package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// encoding/json silently replaces malformed UTF-8 with U+FFFD. A damaged
// primary profile must not hide a valid backup or become the next backup.
func malformedProfileUTF8(t *testing.T, invalid []byte) []byte {
	t.Helper()
	const marker = "INVALID_UTF8_MARKER"
	data := []byte(`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"note":"` + marker + `"}`)
	if !bytes.Contains(data, []byte(marker)) {
		t.Fatal("profile marker missing")
	}
	return bytes.Replace(data, []byte(marker), invalid, 1)
}

func TestLoadProfileRejectsMalformedUTF8(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"invalid-byte", []byte{0xff}},
		{"truncated-sequence", []byte{0xc3}},
		{"bad-continuation", []byte{0xe2, 0x28, 0xa1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, malformedProfileUTF8(t, tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if profile, err := loadProfileFile(path); err == nil {
				t.Fatalf("malformed UTF-8 accepted: %+v", profile.Extra)
			}
		})
	}
}

func TestProfileStoreMalformedUTF8PrimaryRecoversBackup(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	const validBackup = `{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"}}`
	if err := os.WriteFile(primary, malformedProfileUTF8(t, []byte{0xff}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte(validBackup), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	if got.Settings.Server != "original" || got.Settings.Theme != "light" || got.Settings.View != "cards" {
		t.Fatalf("valid backup was masked by invalid UTF-8 primary: %+v", got.Settings)
	}
	// The first save after recovery must not overwrite the valid backup
	// with the malformed primary that caused the recovery.
	if err := store.Replace(got); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(preserved, []byte(validBackup)) {
		t.Fatalf("valid backup overwritten after recovery: %q, %v", preserved, err)
	}
}

func TestLoadProfilePreservesValidUnicode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	const valid = `{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"},"note":"Проверка ✓"}`
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := loadProfileFile(path)
	if err != nil || !bytes.Contains(profile.Extra["note"], []byte("Проверка ✓")) {
		t.Fatalf("valid UTF-8 profile rejected or altered: %+v, %v", profile.Extra, err)
	}
}
