package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.2", "1.0.1", 1},
		{"v1.0.2", "1.0.2", 0},
		{"1.0.2", "1.1.0", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("compareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalizeVersionHumanFormats(t *testing.T) {
	valid := []string{
		"1.1.0.", "1.1.0", "1.1.", "1.1",
		"v1.1.0.", "v1.1.0", "v1.1.", "v1.1",
		"v 1.1.0.", "v 1.1.0", "v 1.1.", "v 1.1",
	}

	for _, input := range valid {
		got, err := normalizeVersion(input)
		if err != nil || got != "1.1.0" {
			t.Fatalf("normalizeVersion(%q)=(%q,%v), want 1.1.0", input, got, err)
		}
	}

	for _, input := range []string{
		"1",
		"v",
		"1.1-beta",
		"v1.1.0-beta",
		"1.1..",
		"version 1.1",
	} {
		if got, err := normalizeVersion(input); err == nil {
			t.Fatalf("normalizeVersion(%q)=%q, want error", input, got)
		}
	}
}

func TestUpdateCheckFindsNewerReleaseWithoutFollowingRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		fmt.Fprintln(w, `{"tag_name":"v9.9.9"}`)
	}))
	defer target.Close()

	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer release.Close()

	client := &http.Client{Timeout: time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	result := checkLatestRelease(context.Background(), client, release.URL, "1.0.2")
	if result.Checked || result.UpdateAvailable || redirected.Load() != 0 {
		t.Fatalf("redirect must be rejected: result=%+v redirected=%d", result, redirected.Load())
	}
}

func TestUpdateCheckValidatesAndBoundsGitHubResponse(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantChecked bool
		wantLatest  string
		wantUpdate  bool
	}{
		{"newer", `{"tag_name":"v1.0.3"}`, true, "1.0.3", true},
		{"same", `{"tag_name":"1.0.2"}`, true, "1.0.2", false},
		{"older", `{"tag_name":"1.0.1"}`, true, "1.0.1", false},
		{"prerelease-like tag rejected", `{"tag_name":"v1.0.3-beta"}`, false, "", false},
		{"malformed", `{`, false, "", false},
		{"oversized", `{"tag_name":"1.0.3","padding":"` + strings.Repeat("x", maxUpdateResponseBytes) + `"}`, false, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Referer") != "" {
					t.Error("update check must not send credentials, cookies or referrer")
				}
				if r.Header.Get("User-Agent") != "IrisOnlineDatabase" {
					t.Errorf("unexpected User-Agent %q", r.Header.Get("User-Agent"))
				}
				if r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
					t.Errorf("unexpected GitHub API version %q", r.Header.Get("X-GitHub-Api-Version"))
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			result := checkLatestRelease(context.Background(), &http.Client{Timeout: time.Second}, server.URL, "1.0.2")
			if result.Checked != tc.wantChecked || result.LatestVersion != tc.wantLatest || result.UpdateAvailable != tc.wantUpdate {
				t.Fatalf("result=%+v", result)
			}
			if result.UpdateAvailable && result.ReleaseURL != githubLatestReleaseURL {
				t.Fatalf("unexpected release URL %q", result.ReleaseURL)
			}
		})
	}
}

func TestUpdateCheckerRunsAtMostOncePerProcess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprintln(w, `{"tag_name":"1.0.3"}`)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.apiURL = server.URL
	checker.client = server.Client()
	const goroutines = 32
	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() { checker.Check(context.Background()); done <- struct{}{} }()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one update request, got %d", calls.Load())
	}
}

func TestUpdateCheckHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	result := checkLatestRelease(ctx, &http.Client{Timeout: 2 * time.Second}, server.URL, "1.0.2")
	if result.Checked || result.UpdateAvailable {
		t.Fatalf("canceled check must fail closed: %+v", result)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("canceled check returned too slowly: %v", elapsed)
	}
}

func TestUpdateCheckRejectsNonGET(t *testing.T) {
	app := &application{updates: newUpdateChecker(), sessions: newSessionManager()}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/api/update-check", nil)
	req.Host = "127.0.0.1:8765"
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestUpdateCheckEndpointUsesBoundedChecker(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		fmt.Fprintln(w, `{"tag_name":"v1.1.1"}`)
	}))
	defer github.Close()
	checker := newUpdateChecker()
	checker.apiURL = github.URL
	checker.client = github.Client()
	app := &application{updates: checker, sessions: newSessionManager()}
	handler := app.routes()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/api/update-check", nil)
	req.Host = "127.0.0.1:8765"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"latestVersion":"1.1.1"`) || !strings.Contains(rec.Body.String(), `"updateAvailable":true`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestUpdateCheckEndpointCancelsWhenApplicationShutsDown(t *testing.T) {
	started := make(chan struct{})
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer github.Close()

	checker := newUpdateChecker()
	checker.apiURL = github.URL
	checker.client = github.Client()
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	app := &application{updates: checker, sessions: newSessionManager(), ctx: appCtx}
	handler := app.routes()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/api/update-check", nil)
	req.Host = "127.0.0.1:8765"
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("external update request did not start")
	}
	start := time.Now()
	appCancel()
	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Fatalf("update handler canceled too slowly: %v", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("update handler did not stop after application shutdown")
	}
}
