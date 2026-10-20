package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The on-disk loader refuses profiles larger than maxProfileBytes. A save
// must not succeed with a payload that cannot be read at the next startup.
func oversizedProfileExtra() map[string]json.RawMessage {
	extra := make(map[string]json.RawMessage, 50)
	value := json.RawMessage(strconv.Quote(strings.Repeat("x", 48<<10)))
	for i := 0; i < 50; i++ {
		extra["extension"+strconv.Itoa(i)] = value
	}
	return extra
}

func profileSizeTestStore(t *testing.T) *profileStore {
	t.Helper()
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	store := &profileStore{
		path:    filepath.Join(root, "profile.json"),
		backup:  filepath.Join(backups, "profile.json.bak"),
		profile: defaultProfile(),
	}
	if err := store.Replace(store.Get()); err != nil {
		t.Fatal(err)
	}
	next := store.Get()
	next.Settings.Theme = "light"
	if err := store.Replace(next); err != nil {
		t.Fatal(err)
	}
	return store
}

func assertProfileDiskUnchanged(t *testing.T, store *profileStore, primary, backup []byte) {
	t.Helper()
	for path, want := range map[string][]byte{store.path: primary, store.backup: backup} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("profile data changed after rejected oversized write: %s (%v)", path, err)
		}
	}
}

func TestProfileReplaceRejectsUnreloadableOversizedPayload(t *testing.T) {
	store := profileSizeTestStore(t)
	primary, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(store.backup)
	if err != nil {
		t.Fatal(err)
	}
	previous := store.Get()
	next := store.Get()
	next.Extra = oversizedProfileExtra()
	encoded, err := json.MarshalIndent(sanitizeProfile(next), "", "  ")
	if err != nil || int64(len(encoded)) <= maxProfileBytes {
		t.Fatalf("invalid oversized fixture: size=%d, err=%v", len(encoded), err)
	}
	if err := store.Replace(next); err == nil {
		t.Fatal("Replace accepted a profile too large for loadProfileFile")
	}
	assertProfileDiskUnchanged(t, store, primary, backup)
	if got := store.Get(); got.Settings != previous.Settings || len(got.Extra) != len(previous.Extra) {
		t.Fatal("Replace changed in-memory profile despite failed write")
	}
}

func TestProfileFlushRejectsUnreloadableOversizedPayload(t *testing.T) {
	store := profileSizeTestStore(t)
	primary, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(store.backup)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.profile.Extra = oversizedProfileExtra()
	store.mu.Unlock()
	if err := store.Flush(); err == nil {
		t.Fatal("Flush accepted a profile too large for loadProfileFile")
	}
	assertProfileDiskUnchanged(t, store, primary, backup)
}

func TestProfileWritesRemainReadableBelowSizeLimit(t *testing.T) {
	store := profileSizeTestStore(t)
	next := store.Get()
	next.Extra = map[string]json.RawMessage{"extension": json.RawMessage(strconv.Quote(strings.Repeat("x", 1024)))}
	if err := store.Replace(next); err != nil {
		t.Fatal(err)
	}
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadProfileFile(store.path)
	if err != nil || len(loaded.Extra) != 1 {
		t.Fatalf("valid profile was not reloadable: %+v, %v", loaded.Extra, err)
	}
}
