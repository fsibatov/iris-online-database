package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCacheBypassesHeaderSensitiveRequests(t *testing.T) {
	for _, header := range []string{"Authorization", "Cookie", "Range", "If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since", "If-Range"} {
		t.Run(header, func(t *testing.T) {
			count := 0
			a := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
			handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count++; fmt.Fprint(w, count) }))
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
				req.Header.Set(header, "test")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			}
			if count != 2 {
				t.Fatalf("%s request unexpectedly cached: calls=%d", header, count)
			}
		})
	}
}

func TestCacheRespectsResponsePrivacyHeaders(t *testing.T) {
	for _, tc := range []struct{ name, value string }{{"Set-Cookie", "session=secret"}, {"Cache-Control", "private, no-store"}, {"Vary", "Accept-Language"}} {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			a := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
			handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				w.Header().Set(tc.name, tc.value)
				fmt.Fprint(w, count)
			}))
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/items", nil))
			}
			if count != 2 {
				t.Fatalf("private response unexpectedly cached: calls=%d", count)
			}
		})
	}
}

func TestCacheStillCachesPublicCatalogResponses(t *testing.T) {
	calls := 0
	a := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = fmt.Fprint(w, "public")
	}))
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/items", nil))
		if rec.Body.String() != "public" {
			t.Fatalf("wrong body: %q", rec.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("public response not cached: calls=%d", calls)
	}
}
