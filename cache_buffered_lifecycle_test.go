package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A backend can finish after its request was canceled. Buffered responses
// must be discarded before their headers, body, or cache entry are published.
func TestBufferedCacheResponseDiscardedAfterRequestCancellation(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte(`{"private":"should-not-escape"}`))
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(ctx)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() { handler.ServeHTTP(rec, req); close(finished) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("backend did not start")
	}
	cancel()
	close(release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled backend did not finish")
	}
	if rec.Code != http.StatusRequestTimeout || strings.Contains(rec.Body.String(), "should-not-escape") {
		t.Fatalf("canceled buffered response leaked: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("cancellation response lost security headers: %v", rec.Header())
	}
	if app.cache.lru.Len() != 0 {
		t.Fatalf("canceled response was cached: entries=%d", app.cache.lru.Len())
	}
}

// Shutdown has priority over request cancellation, including when shutdown
// starts while a catalog handler is still producing a buffered response.
func TestBufferedCacheResponseDiscardedAfterShutdown(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"private":"should-not-escape"}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() { handler.ServeHTTP(rec, req); close(finished) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("backend did not start")
	}
	app.closing.Store(true)
	app.cache.Close()
	close(release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown backend did not finish")
	}
	if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "should-not-escape") {
		t.Fatalf("shutdown buffered response leaked: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("shutdown response lost security headers: %v", rec.Header())
	}
	if app.cache.lru.Len() != 0 {
		t.Fatalf("shutdown response was cached: entries=%d", app.cache.lru.Len())
	}
}
