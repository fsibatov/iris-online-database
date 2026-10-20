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

// JSON null silently zeroes a struct or string in encoding/json. For known
// settings fields, that would discard persisted preferences without an error.
func TestProfileRejectsNullSettingsValues(t *testing.T) {
	cases := []struct{ name, body string }{
		{"settings-null", `{"schemaVersion":1,"settings":null}`},
		{"settings-alias-null", `{"schemaVersion":1,"Settings":null}`},
		{"server-null", `{"schemaVersion":1,"settings":{"server":null,"theme":"dark","view":"list"}}`},
		{"theme-alias-null", `{"schemaVersion":1,"settings":{"server":"original","Theme":null,"view":"cards"}}`},
		{"view-null", `{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":null}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var direct userProfile
			if err := json.Unmarshal([]byte(tc.body), &direct); err == nil {
				t.Errorf("null known preference accepted by profile decoder: %+v", direct.Settings)
			}
			req := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			var incoming userProfile
			if decodeJSONRequest(recorder, req, &incoming, 1<<20) || recorder.Code != http.StatusBadRequest {
				t.Errorf("null known preference accepted by API: status=%d", recorder.Code)
			}
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadProfileFile(path); err == nil {
				t.Error("null known preference accepted from disk")
			}
		})
	}
}

func TestProfileStoreRecoversBackupAfterNullSettings(t *testing.T) {
	cases := []struct{ name, body string }{
		{"settings-null", `{"schemaVersion":1,"settings":null}`},
		{"nested-null", `{"schemaVersion":1,"settings":{"server":null,"theme":"light","view":"cards"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			backups := filepath.Join(root, "Backups")
			if err := os.MkdirAll(backups, 0o700); err != nil {
				t.Fatal(err)
			}
			primary := filepath.Join(root, "profile.json")
			backup := filepath.Join(backups, "profile.json.bak")
			if err := os.WriteFile(primary, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(backup, []byte(`{"schemaVersion":1,"settings":{"server":"original","theme":"dark","view":"cards"}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := newProfileStore(appPaths{Profile: primary, Backups: backups})
			if err != nil {
				t.Fatal(err)
			}
			recovered := store.Get()
			if recovered.Settings.Server != "original" || recovered.Settings.Theme != "dark" || recovered.Settings.View != "cards" {
				t.Fatalf("valid backup ignored: %+v", recovered.Settings)
			}
			recovered.Settings.Theme = "light"
			if err := store.Replace(recovered); err != nil {
				t.Fatal(err)
			}
			preserved, err := loadProfileFile(backup)
			if err != nil || preserved.Settings.Server != "original" || preserved.Settings.Theme != "dark" {
				t.Fatalf("backup corrupted during recovery: %+v, %v", preserved.Settings, err)
			}
		})
	}
}

func TestProfileNullSettingsValidationPreservesLegacyAliases(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"Settings":{"SERVER":"original","Theme":"light","VIEW":"cards"},"custom":true}`,
		`{"schemaVersion":1,"custom":true}`,
	} {
		var profile userProfile
		if err := json.Unmarshal([]byte(body), &profile); err != nil {
			t.Fatalf("valid legacy profile rejected: %v", err)
		}
		if profile.Extra["custom"] == nil {
			t.Fatalf("unknown metadata lost: %+v", profile.Extra)
		}
	}
}
