package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Shutdown must release cache-miss followers even if the original producer
// has not returned and the caller did not supply a cancellation context.
func TestResponseCacheCloseReleasesInflightWaiters(t *testing.T) {
	cache := newResponseCache(8, 1<<20, time.Minute)
	ready, release := cache.beginLoad("/api/items?server=original")
	if release == nil || ready == nil {
		t.Fatal("first cache miss did not become leader")
	}
	follower, followerRelease := cache.beginLoad("/api/items?server=original")
	if followerRelease != nil || follower != ready {
		t.Fatal("concurrent cache miss did not join leader")
	}

	cache.Close()
	select {
	case <-follower:
	default:
		t.Fatal("Close left cache-miss follower blocked behind a running producer")
	}
	cache.mu.Lock()
	remaining := len(cache.inflight)
	cache.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("Close retained %d inflight request keys", remaining)
	}

	// A late producer must not panic by closing the same channel twice.
	release()
	release() // defensive idempotence
}

func TestResponseCacheCloseRejectsNewFlights(t *testing.T) {
	cache := newResponseCache(8, 1<<20, time.Minute)
	cache.Close()
	ready, release := cache.beginLoad("/api/items")
	if ready != nil || release != nil {
		if release != nil {
			release()
		}
		t.Fatal("closed cache accepted a new in-flight producer")
	}
	cache.mu.Lock()
	remaining := len(cache.inflight)
	cache.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("closed cache allocated %d in-flight request keys", remaining)
	}
}

func TestClosedCacheRejectsNewCatalogRequests(t *testing.T) {
	cache := newResponseCache(8, 1<<20, time.Minute)
	app := &application{cache: cache}
	cache.Close()
	calls := 0
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte("should not run"))
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/items", nil))
	if response.Code != http.StatusServiceUnavailable || calls != 0 {
		t.Fatalf("closed cache served request: status=%d backend calls=%d", response.Code, calls)
	}
}
