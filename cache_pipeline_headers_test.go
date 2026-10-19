package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The outer security middleware supplies a default no-store. An explicit
// Cache-Control from the endpoint must replace that default on both a cache
// miss and a hit, not be appended only to the initial response.
func TestPipelineCacheControlIdenticalOnMissAndHit(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte("catalog"))
	}))
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
		request.Host = "wails.localhost"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != "catalog" {
			t.Fatalf("request %d: status=%d body=%q", i, response.Code, response.Body.String())
		}
		if values := response.Result().Header.Values("Cache-Control"); len(values) != 1 || values[0] != "public, max-age=60" {
			t.Fatalf("request %d: inconsistent Cache-Control fields: %q", i, values)
		}
		if response.Result().Header.Get("Content-Security-Policy") == "" {
			t.Fatalf("request %d: security headers were dropped", i)
		}
	}
	if calls != 1 {
		t.Fatalf("cache unexpectedly missed: backend calls=%d", calls)
	}
}

// Large responses bypass the cache, but must still preserve the endpoint's
// explicit cache policy without retaining the security middleware's default.
func TestPipelineStreamingCacheControlNotDuplicated(t *testing.T) {
	app := &application{cache: newResponseCache(8, 64, time.Minute)}
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write([]byte(strings.Repeat("x", 128)))
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	request.Host = "wails.localhost"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() != 128 {
		t.Fatalf("streaming response corrupted: status=%d size=%d", response.Code, response.Body.Len())
	}
	if values := response.Result().Header.Values("Cache-Control"); len(values) != 1 || values[0] != "private, no-store" {
		t.Fatalf("streaming response duplicated Cache-Control: %q", values)
	}
	if response.Result().Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers were lost while streaming")
	}
}

// Endpoints without an explicit cache policy must keep the outer middleware's
// no-store default, even when their response is reused by the internal cache.
func TestPipelineDefaultCacheControlRemainsNoStore(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte("catalog"))
	}))
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
		request.Host = "wails.localhost"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if values := response.Result().Header.Values("Cache-Control"); len(values) != 1 || values[0] != "no-store" {
			t.Fatalf("request %d: default no-store was lost: %q", i, values)
		}
	}
	if calls != 1 {
		t.Fatalf("internal cache unexpectedly missed: backend calls=%d", calls)
	}
}
