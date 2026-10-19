package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A short origin freshness lifetime must not silently become the application's
// five-minute in-process cache TTL. No sleeps: inspect the stored deadline.
func TestResponseCacheCapsLifetimeToMaxAge(t *testing.T) {
	for _, directive := range []string{"public, max-age=1", `public, MAX-AGE="1"`} {
		t.Run(directive, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 1<<20, 5*time.Minute)}
			calls := 0
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Cache-Control", directive)
				fmt.Fprint(w, calls)
			}))
			request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
			handler.ServeHTTP(httptest.NewRecorder(), request)
			key := cacheKeyForRequest(request)
			app.cache.mu.Lock()
			element := app.cache.entries[key]
			var remaining time.Duration
			if element != nil {
				remaining = time.Until(element.Value.(*cacheEntry).expiresAt)
			}
			app.cache.mu.Unlock()
			if element == nil || remaining <= 0 || remaining > time.Second {
				t.Fatalf("max-age=1: cache deadline in %v (expected <=1s)", remaining)
			}
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if calls != 1 {
				t.Fatalf("fresh response not reused: %d calls", calls)
			}
		})
	}
}

func TestResponseCacheRejectsInvalidMaxAge(t *testing.T) {
	for _, directive := range []string{
		"public, max-age=invalid", "public, max-age=-1", "public, max-age=", "public, max-age=1, max-age=2",
		"public, max-age=18446744073709551616", "public, max-age=0",
	} {
		t.Run(directive, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 1<<20, 5*time.Minute)}
			calls := 0
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Cache-Control", directive)
				fmt.Fprint(w, calls)
			}))
			for i := 0; i < 2; i++ {
				handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/items", nil))
			}
			if calls != 2 {
				t.Fatalf("invalid/ambiguous max-age response cached: calls=%d", calls)
			}
		})
	}
}

func TestResponseCacheKeepsShorterConfiguredTTL(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, 2*time.Second)}
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		fmt.Fprint(w, "ok")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	app.cache.mu.Lock()
	element := app.cache.entries[cacheKeyForRequest(request)]
	var remaining time.Duration
	if element != nil {
		remaining = time.Until(element.Value.(*cacheEntry).expiresAt)
	}
	app.cache.mu.Unlock()
	if element == nil || remaining <= 0 || remaining > 2*time.Second {
		t.Fatalf("configured TTL exceeded: %v", remaining)
	}
}
