package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func jsonResponse(status int, body string, request *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

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
	for _, input := range []string{"1", "v", "1.1-beta", "v1.1.0-beta", "1.1..", "version 1.1"} {
		if got, err := normalizeVersion(input); err == nil {
			t.Fatalf("normalizeVersion(%q)=%q, want error", input, got)
		}
	}
}

func TestUpdateCheckFindsNewerReleaseWithoutFollowingAPIRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"v9.9.9"}`)
	}))
	defer target.Close()

	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer release.Close()

	client := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	result := checkLatestRelease(context.Background(), client, release.URL, "1.0.2")
	if result.Checked || result.UpdateAvailable || redirected.Load() != 0 || result.Failure != updateFailureService {
		t.Fatalf("redirect must be rejected: result=%+v redirected=%d", result, redirected.Load())
	}
}

func TestUpdateCheckValidatesAndBoundsGitHubAPIResponse(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantChecked bool
		wantLatest  string
		wantUpdate  bool
		wantFailure string
	}{
		{"newer", `{"tag_name":"v1.0.3"}`, true, "1.0.3", true, ""},
		{"same", `{"tag_name":"1.0.2"}`, true, "1.0.2", false, ""},
		{"older", `{"tag_name":"1.0.1"}`, true, "1.0.1", false, ""},
		{"prerelease-like tag rejected", `{"tag_name":"v1.0.3-beta"}`, false, "", false, updateFailureInvalidResponse},
		{"malformed", `{`, false, "", false, updateFailureInvalidResponse},
		{"oversized", `{"tag_name":"1.0.3","padding":"` + strings.Repeat("x", maxUpdateResponseBytes) + `"}`, false, "", false, updateFailureInvalidResponse},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Referer") != "" {
					t.Error("update check must not send credentials, cookies or referrer")
				}
				if r.Header.Get("User-Agent") != "IrisOnlineDatabase/"+appVersion {
					t.Errorf("unexpected User-Agent %q", r.Header.Get("User-Agent"))
				}
				if r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
					t.Errorf("unexpected GitHub API version %q", r.Header.Get("X-GitHub-Api-Version"))
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			result := checkLatestRelease(context.Background(), &http.Client{Timeout: time.Second}, server.URL, "1.0.2")
			if result.Checked != tc.wantChecked || result.LatestVersion != tc.wantLatest || result.UpdateAvailable != tc.wantUpdate || result.Failure != tc.wantFailure {
				t.Fatalf("result=%+v", result)
			}
			if result.UpdateAvailable && result.ReleaseURL != githubReleaseTagPrefix+tc.wantLatest {
				t.Fatalf("unexpected release URL %q", result.ReleaseURL)
			}
		})
	}
}

func TestUpdateCheckRejectsUnexpectedAPIContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
	}))
	defer server.Close()
	result := checkLatestRelease(context.Background(), server.Client(), server.URL, "1.0.0")
	if result.Checked || result.Failure != updateFailureInvalidResponse {
		t.Fatalf("unexpected content type must fail closed: %+v", result)
	}
}

func TestUpdateCheckUsesTrustedGitHubWebRedirect(t *testing.T) {
	var apiCalls atomic.Int32
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Hostname() {
			case "github.com":
				if r.Method != http.MethodHead {
					t.Fatalf("web release check method=%s want HEAD", r.Method)
				}
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{"https://github.com/fsibatov/iris-online-database/releases/tag/v2.1.0"}},
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			case "api.github.com":
				apiCalls.Add(1)
				return jsonResponse(http.StatusOK, `{"tag_name":"v9.9.9"}`, r), nil
			default:
				t.Fatalf("unexpected host %s", r.URL.Hostname())
				return nil, nil
			}
		}),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	result := checkLatestReleaseSources(context.Background(), client, githubLatestReleaseURL, githubLatestReleaseAPI, "2.0.5")
	if !result.Checked || !result.UpdateAvailable || result.LatestVersion != "2.1.0" || result.ReleaseURL != githubReleaseTagPrefix+"2.1.0" {
		t.Fatalf("unexpected web release result: %+v", result)
	}
	if apiCalls.Load() != 0 {
		t.Fatalf("API fallback must not run after successful web check, calls=%d", apiCalls.Load())
	}
}

func TestUpdateCheckRejectsUntrustedGitHubWebRedirect(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"https://example.com/fsibatov/iris-online-database/releases/tag/v9.9.9"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}),
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	result := checkLatestReleaseSources(context.Background(), client, githubLatestReleaseURL, "", "2.0.5")
	if result.Checked || result.Failure != updateFailureInvalidResponse {
		t.Fatalf("untrusted redirect must fail closed: %+v", result)
	}
}

func TestUpdateCheckRejectsGitHubRedirectOnNonStandardPort(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"https://github.com:444/fsibatov/iris-online-database/releases/tag/v9.9.9"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}),
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	result := checkLatestReleaseSources(context.Background(), client, githubLatestReleaseURL, "", "2.0.5")
	if result.Checked || result.Failure != updateFailureInvalidResponse {
		t.Fatalf("non-standard redirect port must fail closed: %+v", result)
	}
}

func TestUpdateCheckFallsBackToAPIWhenGitHubWebIsUnavailable(t *testing.T) {
	var apiCalls atomic.Int32
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Hostname() == "github.com" {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			}
			apiCalls.Add(1)
			return jsonResponse(http.StatusOK, `{"tag_name":"v2.0.4"}`, r), nil
		}),
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	result := checkLatestReleaseSources(context.Background(), client, githubLatestReleaseURL, githubLatestReleaseAPI, "2.0.5")
	if !result.Checked || result.UpdateAvailable || result.LatestVersion != "2.0.4" || result.Failure != "" {
		t.Fatalf("API fallback failed: %+v", result)
	}
	if apiCalls.Load() != 1 {
		t.Fatalf("API fallback calls=%d want=1", apiCalls.Load())
	}
}

func TestUpdateCheckReportsRateLimitAndRetryWindow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprint(time.Now().Add(2*time.Minute).Unix()))
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer server.Close()
	result := checkLatestRelease(context.Background(), server.Client(), server.URL, "2.0.5")
	if result.Checked || result.Failure != updateFailureRateLimited || result.RetryAfterSeconds < 100 || result.RetryAfterSeconds > 120 {
		t.Fatalf("rate limit not classified: %+v", result)
	}
}

func TestUpdateCheckerCachesAutomaticSuccess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"1.0.3"}`)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = server.URL
	checker.client = server.Client()
	const goroutines = 32
	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() { checker.Check(context.Background(), false); done <- struct{}{} }()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one update request, got %d", calls.Load())
	}
}

func TestUpdateCheckerDoesNotBypassActiveRateLimitOnForcedRefresh(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "120")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = server.URL
	checker.client = server.Client()
	first := checker.Check(context.Background(), false)
	second := checker.Check(context.Background(), true)
	if first.Failure != updateFailureRateLimited || second.Failure != updateFailureRateLimited {
		t.Fatalf("rate limit was not preserved: first=%+v second=%+v", first, second)
	}
	if calls.Load() != 1 {
		t.Fatalf("forced refresh must respect active rate-limit cooldown, calls=%d", calls.Load())
	}
}

func TestUpdateServiceRetryAfterIsNotMisclassifiedAsRateLimit(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Retry-After": []string{"120"}},
	}
	result := classifyUpdateHTTPFailure(response)
	if result.Failure != updateFailureService || result.RetryAfterSeconds != 120 {
		t.Fatalf("service retry window was misclassified: %+v", result)
	}
}

func TestUpdateRateLimitWithoutHeadersUsesSafeMinimumCooldown(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header)}
	result := classifyUpdateHTTPFailure(response)
	if result.Failure != updateFailureRateLimited || result.RetryAfterSeconds != 60 {
		t.Fatalf("unexpected default rate-limit cooldown: %+v", result)
	}
}

func TestUpdateCheckerRespectsServiceRetryAfterOnForcedRefresh(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "120")
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = server.URL
	checker.client = server.Client()
	first := checker.Check(context.Background(), false)
	second := checker.Check(context.Background(), true)
	if first.Failure != updateFailureService || second.Failure != updateFailureService {
		t.Fatalf("service retry window was not preserved: first=%+v second=%+v", first, second)
	}
	if calls.Load() != 1 {
		t.Fatalf("forced refresh must respect service Retry-After, calls=%d", calls.Load())
	}
}

func TestUpdateCheckerRetriesExpiredFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = server.URL
	checker.client = server.Client()
	first := checker.Check(context.Background(), false)
	if first.Failure != updateFailureService {
		t.Fatalf("first failure=%+v", first)
	}
	checker.mu.Lock()
	checker.checkedAt = time.Now().Add(-updateFailureCacheTTL - time.Second)
	checker.mu.Unlock()
	checker.Check(context.Background(), false)
	if calls.Load() != 2 {
		t.Fatalf("expired failure must be retried, calls=%d", calls.Load())
	}
}

func TestUpdateCheckerPreservesLastGoodResultAfterRefreshFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"tag_name":"2.0.4"}`)
			return
		}
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = server.URL
	checker.client = server.Client()
	first := checker.Check(context.Background(), false)
	second := checker.Check(context.Background(), true)
	if !first.Checked || first.LatestVersion != "2.0.4" {
		t.Fatalf("initial result=%+v", first)
	}
	if !second.Checked || !second.Stale || second.LatestVersion != "2.0.4" || second.Failure != updateFailureService {
		t.Fatalf("last good result not preserved: %+v", second)
	}
}

func TestUpdateCheckerCanRefreshManually(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"1.1.0"}`)
	}))
	defer server.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = server.URL
	checker.client = server.Client()
	checker.Check(context.Background(), false)
	checker.Check(context.Background(), false)
	checker.Check(context.Background(), true)
	if calls.Load() != 2 {
		t.Fatalf("expected automatic check plus one manual refresh, got %d requests", calls.Load())
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
	if result.Checked || result.UpdateAvailable || result.Failure != updateFailureCanceled {
		t.Fatalf("canceled check must fail closed: %+v", result)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("canceled check returned too slowly: %v", elapsed)
	}
}

func TestUpdateCheckRejectsNonGET(t *testing.T) {
	app := &application{updates: newUpdateChecker()}
	req := httptest.NewRequest(http.MethodPost, "http://wails.localhost/api/update-check", nil)
	req.Host = "wails.localhost"
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
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tag_name":"v%s"}`+"\n", appVersion)
	}))
	defer github.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = github.URL
	checker.client = github.Client()
	app := &application{updates: checker}
	handler := app.routes()

	req := httptest.NewRequest(http.MethodGet, "http://wails.localhost/api/update-check", nil)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"latestVersion":"`+appVersion+`"`) || !strings.Contains(rec.Body.String(), `"updateAvailable":false`) || strings.Contains(rec.Body.String(), "diagnostic") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestUpdateCheckEndpointRefreshesOnRequest(t *testing.T) {
	var calls atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"v1.1"}`)
	}))
	defer github.Close()
	checker := newUpdateChecker()
	checker.webURL = ""
	checker.apiURL = github.URL
	checker.client = github.Client()
	app := &application{updates: checker}
	handler := app.routes()
	for _, target := range []string{
		"http://wails.localhost/api/update-check",
		"http://wails.localhost/api/update-check?refresh=1",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Host = "wails.localhost"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"latestVersion":"1.1.0"`) {
			t.Fatalf("target=%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("expected forced refresh to make second request, got %d", calls.Load())
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
	checker.webURL = ""
	checker.apiURL = github.URL
	checker.client = github.Client()
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	app := &application{updates: checker, ctx: appCtx}
	handler := app.routes()

	req := httptest.NewRequest(http.MethodGet, "http://wails.localhost/api/update-check", nil)
	req.Host = "wails.localhost"
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
