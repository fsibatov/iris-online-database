package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// net/http commits response headers at WriteHeader. The buffering cache must
// not expose or cache mutations made by an endpoint after that point.
func TestCachePreservesHeadersCommittedBeforeWriteHeader(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.WriteHeader(http.StatusOK)
		// These changes must not reach the client or prevent safe caching.
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Set-Cookie", "late=secret")
		w.Header().Add("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/items", nil)
		r.Host = "wails.localhost"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK || rec.Body.String() != `{"items":[]}` {
			t.Fatalf("request %d: status=%d body=%q", i, rec.Code, rec.Body.String())
		}
		if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("request %d: late Content-Type reached client: %q", i, got)
		}
		if got := rec.Result().Header.Values("Cache-Control"); len(got) != 1 || got[0] != "public, max-age=60" {
			t.Errorf("request %d: late Cache-Control reached client: %q", i, got)
		}
		if got := rec.Result().Header.Get("Set-Cookie"); got != "" {
			t.Errorf("request %d: late cookie reached client: %q", i, got)
		}
		if rec.Result().Header.Get("Content-Security-Policy") == "" {
			t.Fatal("security headers were lost")
		}
	}
	if calls != 1 {
		t.Fatalf("late header mutations prevented cache reuse: backend calls=%d, want 1", calls)
	}
}

func TestStreamingCapturePreservesCommittedHeaders(t *testing.T) {
	app := &application{cache: newResponseCache(8, 64, time.Minute)}
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Add("Cache-Control", "no-store")
		w.Header().Set("Set-Cookie", "late=secret")
		_, _ = w.Write([]byte(strings.Repeat("x", 128)))
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	r.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK || rec.Body.Len() != 128 {
		t.Fatalf("streaming status=%d bytes=%d", rec.Code, rec.Body.Len())
	}
	if got := rec.Result().Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("late Content-Type reached stream: %q", got)
	}
	if got := rec.Result().Header.Values("Cache-Control"); len(got) != 1 || got[0] != "public, max-age=60" {
		t.Errorf("late Cache-Control reached stream: %q", got)
	}
	if got := rec.Result().Header.Get("Set-Cookie"); got != "" {
		t.Errorf("late cookie reached stream: %q", got)
	}
	if rec.Result().Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers were lost")
	}
}

func TestCaptureWriterImplicitWriteCommitsHeaders(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("first")) // Implicit WriteHeader(200).
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Set-Cookie", "late=secret")
		_, _ = w.Write([]byte("second"))
	}))
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/items", nil)
		r.Host = "wails.localhost"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK || rec.Body.String() != "firstsecond" {
			t.Fatalf("request %d: status=%d body=%q", i, rec.Code, rec.Body.String())
		}
		if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("request %d: implicit header snapshot lost: %q", i, got)
		}
		if got := rec.Result().Header.Get("Set-Cookie"); got != "" {
			t.Errorf("request %d: late cookie reached client: %q", i, got)
		}
	}
	if calls != 1 {
		t.Fatalf("late header mutation prevented reuse: calls=%d", calls)
	}
}
