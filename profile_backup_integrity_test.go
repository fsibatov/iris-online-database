package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A corrupt primary must never replace a valid backup during the first
// successful save after recovery. Otherwise the next interrupted save can
// leave both copies unusable.
func TestProfileReplacePreservesValidBackupAfterCorruptPrimary(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	const corrupted = `{"schemaVersion":1,"settings":{"theme":"dark","theme":"light"}}`
	const recovered = `{"schemaVersion":1,"settings":{"server":"original","theme":"dark","view":"cards"}}`
	if err := os.WriteFile(primary, []byte(corrupted), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte(recovered), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	if store.Get().Settings.Server != "original" {
		t.Fatal("failed to recover backup")
	}
	next := store.Get()
	next.Settings.Theme = "light"
	if err := store.Replace(next); err != nil {
		t.Fatal(err)
	}
	got, err := loadProfileFile(backup)
	if err != nil {
		t.Fatalf("valid backup overwritten by corrupt primary: %v", err)
	}
	if got.Settings.Server != "original" || got.Settings.Theme != "dark" {
		t.Fatalf("backup changed after corrupt primary: %+v", got.Settings)
	}
	current, err := loadProfileFile(primary)
	if err != nil || current.Settings.Theme != "light" {
		t.Fatalf("new primary not persisted: %+v, %v", current.Settings, err)
	}
}

func TestProfileReplaceDoesNotCreateBackupFromCorruptPrimary(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	if err := os.WriteFile(primary, []byte(`{"schemaVersion":1,"settings":{"theme":"dark","theme":"light"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Replace(store.Get()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("corrupt primary copied to backup, stat error: %v", err)
	}
}

func TestProfileReplaceStillBacksUpValidPrimary(t *testing.T) {
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(root, "profile.json")
	backup := filepath.Join(backups, "profile.json.bak")
	store := &profileStore{path: primary, backup: backup, profile: defaultProfile()}
	if err := store.Replace(store.Get()); err != nil {
		t.Fatal(err)
	}
	next := store.Get()
	next.Settings.Theme = "light"
	if err := store.Replace(next); err != nil {
		t.Fatal(err)
	}
	old, err := loadProfileFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if old.Settings.Theme != "dark" {
		t.Fatalf("valid primary not backed up: %+v", old.Settings)
	}
}
