package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Cancellation can occur while the buffered response is being committed to
// the caller. That response is already visible, but it must not seed the cache.
type cancelOnResponseWrite struct {
	*httptest.ResponseRecorder
	afterWrite func()
}

func (w *cancelOnResponseWrite) Write(data []byte) (int, error) {
	n, err := w.ResponseRecorder.Write(data)
	w.afterWrite()
	return n, err
}

func TestCanceledDuringResponseCommitDoesNotPopulateCache(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"catalog":"public"}`))
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(ctx)
	first.Host = "wails.localhost"
	firstRecorder := &cancelOnResponseWrite{ResponseRecorder: httptest.NewRecorder(), afterWrite: cancel}
	handler.ServeHTTP(firstRecorder, first)
	if firstRecorder.Code != http.StatusOK || firstRecorder.Body.String() != `{"catalog":"public"}` {
		t.Fatalf("first response: status=%d body=%q", firstRecorder.Code, firstRecorder.Body.String())
	}
	if firstRecorder.Header().Get("Cache-Control") != "no-store" || firstRecorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("committed response lost security headers: %v", firstRecorder.Header())
	}
	if _, ok := app.cache.Get(cacheKeyForRequest(first)); ok {
		t.Fatal("canceled response was cached after its body was committed")
	}

	second := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	second.Host = "wails.localhost"
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, second)
	if secondRecorder.Code != http.StatusOK || secondRecorder.Body.String() != `{"catalog":"public"}` || calls != 2 {
		t.Fatalf("retry: status=%d body=%q backend calls=%d, want 200/public/2", secondRecorder.Code, secondRecorder.Body.String(), calls)
	}
	if _, ok := app.cache.Get(cacheKeyForRequest(second)); !ok {
		t.Fatal("successful retry did not populate the cache")
	}
}

func TestApplicationCanceledDuringResponseCommitDoesNotPopulateCache(t *testing.T) {
	appCtx, stopApp := context.WithCancel(context.Background())
	defer stopApp()
	app := &application{ctx: appCtx, cache: newResponseCache(8, 1<<20, time.Minute)}
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"catalog":"public"}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.Host = "wails.localhost"
	rec := &cancelOnResponseWrite{ResponseRecorder: httptest.NewRecorder(), afterWrite: stopApp}
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("response already committed: status=%d, want 200", rec.Code)
	}
	if _, ok := app.cache.Get(cacheKeyForRequest(req)); ok {
		t.Fatal("application shutdown during response commit left a cached entry")
	}
}

func TestClosingDuringResponseCommitDoesNotPopulateCache(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"catalog":"public"}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.Host = "wails.localhost"
	rec := &cancelOnResponseWrite{ResponseRecorder: httptest.NewRecorder(), afterWrite: func() { app.closing.Store(true) }}
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("response already committed: status=%d, want 200", rec.Code)
	}
	if _, ok := app.cache.Get(cacheKeyForRequest(req)); ok {
		t.Fatal("closing flag set during response commit left a cached entry")
	}
}
