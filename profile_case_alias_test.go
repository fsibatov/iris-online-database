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

// encoding/json matches struct fields case-insensitively. Two differently
// cased names for the same profile field must not silently override settings.
func TestProfileRejectsCaseVariantKnownFieldCollision(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"settings":{"server":"kiss","theme":"dark","view":"list"},"Settings":{"server":"original","theme":"light","view":"cards"}}`,
		`{"Settings":{"server":"original","theme":"light","view":"cards"},"settings":{"server":"kiss","theme":"dark","view":"list"},"schemaVersion":1}`,
		`{"schemaVersion":1,"SchemaVersion":2,"settings":{"server":"kiss","theme":"dark","view":"list"}}`,
	} {
		req := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		var profile userProfile
		if decodeJSONRequest(rec, req, &profile, 1<<20) || rec.Code != http.StatusBadRequest {
			t.Errorf("case-variant duplicate accepted: status=%d, theme=%q, body=%s", rec.Code, profile.Settings.Theme, body)
		}
	}
}

func TestProfileCaseVariantAliasIsCanonicalized(t *testing.T) {
	data := []byte(`{"SchemaVersion":1,"Settings":{"server":"original","theme":"light","view":"cards"},"customFlag":{"enabled":true}}`)
	var profile userProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.SchemaVersion != 1 || profile.Settings.Theme != "light" {
		t.Fatalf("legacy case-insensitive profile fields were lost: %+v", profile)
	}
	if len(profile.Extra) != 1 || profile.Extra["customFlag"] == nil {
		t.Errorf("reserved aliases leaked into Extra: %v", profile.Extra)
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["Settings"]; ok {
		t.Errorf("serialized profile contains shadow Settings field: %s", encoded)
	}
	if _, ok := fields["SchemaVersion"]; ok {
		t.Errorf("serialized profile contains shadow SchemaVersion field: %s", encoded)
	}
	if _, ok := fields["customFlag"]; !ok {
		t.Errorf("unknown forward-compatible field was lost: %s", encoded)
	}
}

func TestProfileExtraCannotShadowKnownFields(t *testing.T) {
	extras := map[string]json.RawMessage{
		"Settings":      json.RawMessage(`{"theme":"dark"}`),
		"SCHEMAVERSION": json.RawMessage(`2`),
		"customFlag":    json.RawMessage(`true`),
	}
	filtered := sanitizeProfileExtra(extras)
	if len(filtered) != 1 || filtered["customFlag"] == nil {
		t.Errorf("sanitized Extra retains case-variant reserved fields: %v", filtered)
	}
	profile := userProfile{SchemaVersion: 1, Settings: userSettings{Theme: "light"}, Extra: extras}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["Settings"] != nil || fields["SCHEMAVERSION"] != nil || fields["customFlag"] == nil {
		t.Errorf("marshal did not protect reserved profile fields: %s", encoded)
	}
}

// A legacy single-case alias must survive an actual atomic profile write and
// reload without a shadow key that can change its value on the next restart.
func TestProfileAliasSurvivesPersistedRoundTrip(t *testing.T) {
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
	var incoming userProfile
	if err := json.Unmarshal([]byte(`{"SchemaVersion":1,"Settings":{"server":"original","theme":"light","view":"cards"},"customFlag":true}`), &incoming); err != nil {
		t.Fatal(err)
	}
	if err := store.Replace(incoming); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadProfileFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Settings.Theme != "light" || reloaded.Settings.Server != "original" {
		t.Errorf("profile changed across disk round-trip: %+v", reloaded.Settings)
	}
	if len(reloaded.Extra) != 1 || reloaded.Extra["customFlag"] == nil {
		t.Errorf("shadow fields persisted to profile: %v", reloaded.Extra)
	}
	saved, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), `"Settings"`) || strings.Contains(string(saved), `"SchemaVersion"`) {
		t.Errorf("profile file contains reserved aliases: %s", saved)
	}
}
