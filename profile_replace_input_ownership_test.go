package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// Replace preserves previously stored unknown fields, but must not insert
// those fields into the caller's map: the input belongs to the caller.
func TestProfileReplaceDoesNotMutateCallerExtraOnSuccess(t *testing.T) {
	store := profileSizeTestStore(t)
	seed := store.Get()
	seed.Extra = map[string]json.RawMessage{"preserved": json.RawMessage(`{"version":1}`)}
	if err := store.Replace(seed); err != nil {
		t.Fatal(err)
	}

	incoming := store.Get()
	incoming.Extra = map[string]json.RawMessage{"incoming": json.RawMessage(`true`)}
	if err := store.Replace(incoming); err != nil {
		t.Fatal(err)
	}
	if len(incoming.Extra) != 1 || !bytes.Equal(incoming.Extra["incoming"], []byte(`true`)) {
		t.Fatalf("Replace modified caller-owned Extra map: %+v", incoming.Extra)
	}
	persisted, err := loadProfileFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Extra) != 2 || persisted.Extra["preserved"] == nil || persisted.Extra["incoming"] == nil {
		t.Fatalf("stored profile did not retain both extensions: %+v", persisted.Extra)
	}
}

func TestProfileReplaceDoesNotMutateCallerExtraOnFailure(t *testing.T) {
	store := profileSizeTestStore(t)
	seed := store.Get()
	seed.Extra = map[string]json.RawMessage{"preserved": json.RawMessage(`true`)}
	if err := store.Replace(seed); err != nil {
		t.Fatal(err)
	}
	primary, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(store.backup)
	if err != nil {
		t.Fatal(err)
	}

	incoming := store.Get()
	incoming.Extra = oversizedProfileExtra()
	before := copyRawMessageMap(incoming.Extra)
	if err := store.Replace(incoming); err == nil {
		t.Fatal("oversized profile unexpectedly accepted")
	}
	if len(incoming.Extra) != len(before) {
		t.Fatalf("failed Replace modified caller-owned Extra map: got %d keys, want %d", len(incoming.Extra), len(before))
	}
	for key, want := range before {
		if !bytes.Equal(incoming.Extra[key], want) {
			t.Fatalf("failed Replace modified caller-owned key %q", key)
		}
	}
	assertProfileDiskUnchanged(t, store, primary, backup)
	if stored := store.Get(); len(stored.Extra) != 1 || stored.Extra["preserved"] == nil {
		t.Fatalf("failed Replace changed stored profile: %+v", stored.Extra)
	}
}

func TestProfileReplaceNilExtraStillRetainsStoredExtensions(t *testing.T) {
	store := profileSizeTestStore(t)
	seed := store.Get()
	seed.Extra = map[string]json.RawMessage{"preserved": json.RawMessage(`true`)}
	if err := store.Replace(seed); err != nil {
		t.Fatal(err)
	}
	incoming := store.Get()
	incoming.Extra = nil
	if err := store.Replace(incoming); err != nil {
		t.Fatal(err)
	}
	if incoming.Extra != nil {
		t.Fatal("Replace modified caller-owned nil Extra map")
	}
	if got := store.Get(); len(got.Extra) != 1 || got.Extra["preserved"] == nil {
		t.Fatalf("stored extension lost: %+v", got.Extra)
	}
}
