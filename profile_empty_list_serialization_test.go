package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Empty profile lists have an array-shaped JSON contract. Copying an empty
// slice into a nil slice turns [] into null in API responses and saved files.
func TestProfileGetPreservesEmptyListArrays(t *testing.T) {
	store := &profileStore{profile: defaultProfile()}
	got := store.Get()
	if got.Favorites == nil || got.History == nil || got.RecentlyViewed == nil {
		t.Fatalf("empty profile lists became nil: favorites=%v history=%v recent=%v", got.Favorites, got.History, got.RecentlyViewed)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"favorites", "history", "recentlyViewed"} {
		if !bytes.Equal(fields[name], []byte("[]")) {
			t.Errorf("GET profile %s=%s, want []", name, fields[name])
		}
	}
}

func TestProfileFlushPreservesEmptyListArrays(t *testing.T) {
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
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"favorites", "history", "recentlyViewed"} {
		if !bytes.Equal(fields[name], []byte("[]")) {
			t.Errorf("flushed profile %s=%s, want []", name, fields[name])
		}
	}
	if _, err := loadProfileFile(store.path); err != nil {
		t.Fatalf("flushed profile cannot be reloaded: %v", err)
	}
}

func TestProfileStillLoadsLegacyNullLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"favorites":null,"history":null,"recentlyViewed":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err != nil {
		t.Fatalf("legacy null lists must remain readable: %v", err)
	}
}
