package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// An in-process cache reused across HTTP requests must respect s-maxage,
// which overrides the shared-cache lifetime allowed by max-age.
func TestCacheSharedMaxAgeZeroBypassesReuse(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60, s-maxage=0")
		_, _ = fmt.Fprint(w, calls)
	}))
	for i, want := range []string{"1", "2"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/items", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != want {
			t.Fatalf("request %d: status=%d body=%q, want %q", i+1, rec.Code, rec.Body.String(), want)
		}
	}
	if calls != 2 {
		t.Fatalf("s-maxage=0 response was cached: backend calls=%d, want 2", calls)
	}
}

func TestCacheSharedMaxAgeCapsTTL(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60, s-maxage=1")
		_, _ = w.Write([]byte("catalog"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	entry, ok := app.cache.Get(cacheKeyForRequest(req))
	if !ok || entry.ttl != time.Second {
		t.Fatalf("shared-cache lifetime: present=%v ttl=%s, want 1s", ok, entry.ttl)
	}
}

func TestCacheInvalidSharedMaxAgeBypassesReuse(t *testing.T) {
	for _, directive := range []string{
		"s-maxage=invalid", "s-maxage=-1", "s-maxage=1, s-maxage=2", "s-maxage=", "s-maxage=18446744073709551616",
	} {
		t.Run(directive, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
			calls := 0
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Cache-Control", "public, max-age=60, "+directive)
				_, _ = w.Write([]byte("catalog"))
			}))
			for i := 0; i < 2; i++ {
				handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/items", nil))
			}
			if calls != 2 {
				t.Fatalf("invalid %q was cached: backend calls=%d, want 2", directive, calls)
			}
		})
	}
}
