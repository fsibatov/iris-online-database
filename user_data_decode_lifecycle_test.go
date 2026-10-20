package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A profile PUT can pass the outer lifecycle guard, then shutdown can start
// while MaxBytesReader is reading the request body. It must not persist the
// decoded profile after the request or application is canceled.
type cancelDuringBodyRead struct {
	reader io.Reader
	cancel func()
	fired  bool
}

func (r *cancelDuringBodyRead) Read(p []byte) (int, error) {
	if !r.fired {
		r.fired = true
		if r.cancel != nil {
			r.cancel()
		}
	}
	return r.reader.Read(p)
}

func newLifecycleProfileTestApp(t *testing.T) (*application, string) {
	t.Helper()
	root := t.TempDir()
	backupDir := filepath.Join(root, "Backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(root, "profile.json")
	store := &profileStore{path: profilePath, backup: filepath.Join(backupDir, "profile.json.bak"), profile: defaultProfile()}
	return &application{profile: store, cache: newResponseCache(8, 1<<20, time.Minute)}, profilePath
}

func TestUserDataPUTRejectsCancellationDuringDecode(t *testing.T) {
	for _, tc := range []struct {
		mode       string
		wantStatus int
	}{
		{"request", http.StatusRequestTimeout},
		{"application_context", http.StatusServiceUnavailable},
		{"closing", http.StatusServiceUnavailable},
		{"both", http.StatusServiceUnavailable},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			app, path := newLifecycleProfileTestApp(t)
			appCtx, cancelApp := context.WithCancel(context.Background())
			defer cancelApp()
			app.ctx = appCtx
			requestCtx, cancelRequest := context.WithCancel(context.Background())
			defer cancelRequest()
			interrupt := cancelRequest
			switch tc.mode {
			case "application_context":
				interrupt = cancelApp
			case "closing":
				interrupt = func() { app.closing.Store(true) }
			case "both":
				interrupt = func() { cancelRequest(); cancelApp() }
			}
			body := `{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"}}`
			reader := &cancelDuringBodyRead{reader: strings.NewReader(body), cancel: interrupt}
			req := httptest.NewRequest(http.MethodPut, "/api/user-data", reader).WithContext(requestCtx)
			req.Host = "wails.localhost"
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			app.withAPIRequestPipeline(http.HandlerFunc(app.handleUserData)).ServeHTTP(rec, req)
			if !reader.fired {
				t.Fatal("body was not read after the initial lifecycle guard")
			}
			if rec.Code != tc.wantStatus {
				t.Errorf("status=%d, want %d", rec.Code, tc.wantStatus)
			}
			if got := app.profile.Get().Settings.Theme; got != "dark" {
				t.Errorf("canceled PUT replaced saved theme with %q", got)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("canceled PUT wrote profile file: stat err=%v", err)
			}
		})
	}
}

func TestUserDataPUTAllowsActiveDecode(t *testing.T) {
	app, path := newLifecycleProfileTestApp(t)
	app.ctx = context.Background()
	body := `{"schemaVersion":1,"settings":{"server":"original","theme":"light","view":"cards"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(body))
	req.Host = "wails.localhost"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.withAPIRequestPipeline(http.HandlerFunc(app.handleUserData)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || app.profile.Get().Settings.Theme != "light" {
		t.Fatalf("active PUT failed: status=%d theme=%q body=%q", rec.Code, app.profile.Get().Settings.Theme, rec.Body.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active PUT did not persist profile: %v", err)
	}
}
