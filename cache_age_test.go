package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// A cache hit must not give the browser a fresh max-age window. Age reports
// time already spent in the in-process cache, including for public responses.
func TestCachedPublicResponseReportsAge(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, 2*time.Minute)}
	calls := 0
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=120")
		_, _ = fmt.Fprint(w, "catalog")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request)
	if first.Header().Get("Age") != "" {
		t.Fatal("origin response unexpectedly gained an Age header")
	}

	// Simulate 45 seconds of elapsed cache lifetime without a slow sleep.
	app.cache.mu.Lock()
	element := app.cache.entries[cacheKeyForRequest(request)]
	if element == nil {
		app.cache.mu.Unlock()
		t.Fatal("public response was not cached")
	}
	element.Value.(*cacheEntry).expiresAt = time.Now().Add(75 * time.Second)
	app.cache.mu.Unlock()

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)
	if calls != 1 || second.Body.String() != "catalog" {
		t.Fatalf("cache miss or body mismatch: calls=%d body=%q", calls, second.Body.String())
	}
	if got := second.Header().Get("Cache-Control"); got != "public, max-age=120" {
		t.Fatalf("Cache-Control changed: %q", got)
	}
	age, err := strconv.Atoi(second.Header().Get("Age"))
	if err != nil || age < 44 || age > 46 {
		t.Fatalf("Age=%q, expected approximately 45 seconds (err=%v)", second.Header().Get("Age"), err)
	}
}
