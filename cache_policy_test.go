package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCacheRequestDirectivesBypassCache(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"Cache-Control", "no-cache"},
		{"Cache-Control", "max-age=0"},
		{"Cache-Control", "no-store"},
		{"Pragma", "no-cache"},
	} {
		t.Run(tc.name+"/"+tc.value, func(t *testing.T) {
			calls := 0
			app := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				fmt.Fprint(w, calls)
			}))
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
				req.Header.Set(tc.name, tc.value)
				handler.ServeHTTP(httptest.NewRecorder(), req)
			}
			if calls != 2 {
				t.Fatalf("explicit non-cache request was cached: backend calls=%d", calls)
			}
		})
	}
}

func TestCachePrivateDirectiveVariants(t *testing.T) {
	for _, value := range []string{"private=\"Set-Cookie\"", "No-Cache=\"Expires\"", "MAX-AGE = \"0\"", "max-age=00", "public, NO-STORE"} {
		if !hasPrivateCacheDirective(value) {
			t.Errorf("not recognized as private: %q", value)
		}
	}
	for _, value := range []string{"public, max-age=60", "s-maxage=10", "immutable"} {
		if hasPrivateCacheDirective(value) {
			t.Errorf("incorrect private cache directive: %q", value)
		}
	}
}

func TestResponseCacheSeparatesAcceptedRepresentations(t *testing.T) {
	for _, name := range []string{"Accept", "Accept-Language", "Accept-Encoding", "Accept-Charset"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			app := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				_, _ = fmt.Fprint(w, r.Header.Get(name))
			}))
			for _, value := range []string{"first", "second", "first", "second"} {
				req := httptest.NewRequest("GET", "/api/items?server=original", nil)
				req.Header.Set(name, value)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Body.String() != value {
					t.Fatalf("%s %q returned %q", name, value, rec.Body.String())
				}
			}
			if calls != 2 {
				t.Fatalf("expected separate keys per %s value; calls=%d", name, calls)
			}
		})
	}
}

func TestCacheOversizedRepresentationHeadersBypassCache(t *testing.T) {
	calls := 0
	app := &application{cache: newResponseCache(16, 1<<20, time.Minute)}
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = fmt.Fprint(w, calls)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/items", nil)
		req.Header.Set("Accept-Language", strings.Repeat("a", maxCacheKeyBytes))
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	if calls != 2 {
		t.Fatalf("oversized request header was cached: calls=%d", calls)
	}
}

func TestCacheResponsePragmaIsNotCached(t *testing.T) {
	calls := 0
	app := &application{cache: newResponseCache(16, 1<<20, time.Minute)}
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Pragma", "no-cache")
		_, _ = fmt.Fprint(w, calls)
	}))
	for i := 0; i < 2; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/items", nil))
	}
	if calls != 2 {
		t.Fatalf("Pragma response was cached: calls=%d", calls)
	}
}

func TestCachedResponsePreservesSafeMetadata(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Language", "ru")
		w.Header().Set("ETag", `"example"`)
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 00:00:00 GMT")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/items", nil))
		for name, want := range map[string]string{
			"Cache-Control":    "public, max-age=60",
			"Content-Language": "ru",
			"ETag":             `"example"`,
			"Last-Modified":    "Mon, 19 Oct 2026 00:00:00 GMT",
			"Content-Type":     "application/json",
		} {
			if got := rec.Header().Get(name); got != want {
				t.Errorf("request=%d %s=%q want=%q", i, name, got, want)
			}
		}
	}
	if calls != 1 {
		t.Fatalf("expected cached second response; calls=%d", calls)
	}
}
