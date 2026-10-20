package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The JSON decoder matches nested settings fields case-insensitively.
// Two aliases for one preference must never silently overwrite each other.
func TestProfileRejectsNestedSettingsAliases(t *testing.T) {
	cases := []struct{ name, body string }{
		{"theme-first", `{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","Theme":"light","view":"list"}}`},
		{"theme-reversed", `{"schemaVersion":1,"settings":{"Theme":"light","theme":"dark","server":"kiss","view":"list"}}`},
		{"server", `{"schemaVersion":1,"Settings":{"SERVER":"original","server":"kiss","theme":"dark","view":"list"}}`},
		{"view", `{"schemaVersion":1,"settings":{"view":"list","VIEW":"cards","server":"kiss","theme":"dark"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var direct userProfile
			if err := json.Unmarshal([]byte(tc.body), &direct); err == nil {
				t.Errorf("ambiguous nested settings accepted by profile decoder: %+v", direct.Settings)
			}
			req := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			var incoming userProfile
			if decodeJSONRequest(rec, req, &incoming, 1<<20) || rec.Code != http.StatusBadRequest {
				t.Errorf("ambiguous nested settings accepted by API: status=%d, settings=%+v", rec.Code, incoming.Settings)
			}
		})
	}
}

func TestProfileNestedSettingsAliasDiskRoundTrip(t *testing.T) {
	// A single legacy alias is valid; only conflicting aliases are rejected.
	const body = `{"SchemaVersion":1,"Settings":{"SERVER":"original","Theme":"light","VIEW":"cards"},"customFlag":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	var incoming userProfile
	if !decodeJSONRequest(rec, req, &incoming, 1<<20) {
		t.Fatalf("valid legacy nested aliases rejected: HTTP %d %s", rec.Code, rec.Body.String())
	}
	root := t.TempDir()
	backups := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	store := &profileStore{path: filepath.Join(root, "profile.json"), backup: filepath.Join(backups, "profile.json.bak"), profile: defaultProfile()}
	if err := store.Replace(incoming); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadProfileFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Settings.Server != "original" || loaded.Settings.Theme != "light" || loaded.Settings.View != "cards" || loaded.Extra["customFlag"] == nil {
		t.Fatalf("valid legacy aliases lost during profile round-trip: %+v", loaded)
	}
	saved, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{`"SERVER"`, `"Theme"`, `"VIEW"`, `"Settings"`, `"SchemaVersion"`} {
		if strings.Contains(string(saved), alias) {
			t.Errorf("saved profile retains alias %s", alias)
		}
	}
}
