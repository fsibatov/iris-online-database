package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Cache hits bypass the backend limiter, so cancellation must be checked
// by the outer API guard before a cached response can be replayed.
func TestCanceledCachedAPIRequestIsRejected(t *testing.T) {
	app := &application{cache: newResponseCache(16, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"server":"original"}`))
	}))
	makeRequest := func(ctx context.Context) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil)
		req.Host = "wails.localhost"
		return req.WithContext(ctx)
	}
	warm := httptest.NewRecorder()
	handler.ServeHTTP(warm, makeRequest(context.Background()))
	if warm.Code != http.StatusOK || calls != 1 {
		t.Fatalf("cache warm-up: status=%d backend calls=%d", warm.Code, calls)
	}
	if len(app.cache.entries) != 1 {
		t.Fatal("expected warm-up response to be cached")
	}

	for i := 0; i < 64; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, makeRequest(ctx))
		if response.Code != http.StatusRequestTimeout {
			t.Fatalf("canceled cached request %d: status=%d, want 408", i, response.Code)
		}
		if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("canceled cached request %d lost security headers", i)
		}
		if response.Header().Get("Age") != "" || response.Body.String() == `{"server":"original"}` {
			t.Fatalf("canceled cached request %d replayed cached content", i)
		}
	}
	if calls != 1 {
		t.Fatalf("canceled cache hits invoked backend %d extra times", calls-1)
	}
	active := httptest.NewRecorder()
	handler.ServeHTTP(active, makeRequest(context.Background()))
	if active.Code != http.StatusOK || active.Body.String() != `{"server":"original"}` || calls != 1 {
		t.Fatalf("active cache hit: status=%d body=%q backend calls=%d", active.Code, active.Body.String(), calls)
	}
}

// Shutdown must keep its existing 503 precedence, even for a request whose
// context is canceled and whose response is already available in the cache.
func TestShutdownPrecedesCanceledCachedAPIRequest(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte("cached catalog"))
	}))
	request := func(ctx context.Context) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/items?server=kiss", nil).WithContext(ctx)
		r.Host = "wails.localhost"
		return r
	}
	handler.ServeHTTP(httptest.NewRecorder(), request(context.Background()))
	if calls != 1 || len(app.cache.entries) != 1 {
		t.Fatal("cache warm-up failed")
	}
	app.closing.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request(ctx))
	if response.Code != http.StatusServiceUnavailable || calls != 1 {
		t.Fatalf("shutdown response: status=%d backend calls=%d", response.Code, calls)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("shutdown response lost no-store")
	}
}
