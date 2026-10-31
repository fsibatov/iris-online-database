package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A producer that was already executing when shutdown began must not restore
// cached data after shutdown has cleared the in-process cache.
func TestShutdownDoesNotRepopulateResponseCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &application{ctx: ctx, cancel: cancel, cache: newResponseCache(8, 1<<20, time.Minute)}
	started := make(chan struct{})
	release := make(chan struct{})
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = fmt.Fprint(w, "response")
	}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/items", nil))
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("backend did not start")
	}
	if err := app.shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("backend did not finish")
	}
	app.cache.mu.Lock()
	defer app.cache.mu.Unlock()
	if len(app.cache.entries) != 0 || app.cache.bytes != 0 {
		t.Fatalf("shutdown cache was repopulated: entries=%d bytes=%d", len(app.cache.entries), app.cache.bytes)
	}
}

// Closing is terminal; unlike Clear, it rejects late writes and reads.
func TestResponseCacheCloseIsTerminal(t *testing.T) {
	cache := newResponseCache(8, 1<<20, time.Minute)
	cache.Put("before", http.StatusOK, nil, []byte("data"))
	if _, ok := cache.Get("before"); !ok {
		t.Fatal("pre-shutdown entry missing")
	}
	cache.Close()
	cache.Put("after", http.StatusOK, nil, []byte("data"))
	if _, ok := cache.Get("after"); ok {
		t.Fatal("closed cache accepted a new entry")
	}
	cache.Clear()
	cache.Put("after-clear", http.StatusOK, nil, []byte("data"))
	if _, ok := cache.Get("after-clear"); ok {
		t.Fatal("Clear reopened a closed cache")
	}
	cache.Close() // idempotent

	reusable := newResponseCache(8, 1<<20, time.Minute)
	reusable.Put("old", http.StatusOK, nil, []byte("data"))
	reusable.Clear()
	reusable.Put("new", http.StatusOK, nil, []byte("data"))
	if _, ok := reusable.Get("new"); !ok {
		t.Fatal("Clear unexpectedly disabled a reusable cache")
	}
}
