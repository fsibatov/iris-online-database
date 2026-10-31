package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Multiple Cache-Control fields are semantically equivalent to one comma-joined
// field. A later no-store directive must never be lost on cache replay.
func TestCacheRejectsRepeatedCacheControlWithNoStore(t *testing.T) {
	for _, directives := range [][]string{
		{"public, max-age=60", "no-store"},
		{"no-store", "public, max-age=60"},
	} {
		calls := 0
		app := &application{cache: newResponseCache(16, 1<<20, time.Minute)}
		handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			for _, directive := range directives {
				w.Header().Add("Cache-Control", directive)
			}
			_, _ = fmt.Fprint(w, calls)
		}))
		for i := 1; i <= 2; i++ {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/items", nil))
			if response.Body.String() != fmt.Sprint(i) {
				t.Fatalf("directives=%q: response %d reused private body %q", directives, i, response.Body.String())
			}
			if got := response.Header().Values("Cache-Control"); len(got) != 2 || got[0] != directives[0] || got[1] != directives[1] {
				t.Fatalf("directives=%q: response headers changed: %q", directives, got)
			}
		}
		if calls != 2 {
			t.Fatalf("directives=%q: no-store response cached: %d calls", directives, calls)
		}
	}
}

// Cache replay must not silently drop headers outside the supported metadata
// set, or collapse multiple fields into one value.
func TestCacheBypassesUnrepresentableResponseHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
	}{
		{"Content-Disposition", []string{"attachment; filename=items.json"}},
		{"Expires", []string{"Mon, 19 Oct 2026 00:00:00 GMT"}},
		{"X-Request-ID", []string{"request-1"}},
		{"Content-Language", []string{"ru", "en"}},
		{"ETag", []string{`"first"`, `"second"`}},
	} {
		t.Run(tc.name+fmt.Sprint(len(tc.values)), func(t *testing.T) {
			calls := 0
			app := &application{cache: newResponseCache(16, 1<<20, time.Minute)}
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				for _, value := range tc.values {
					w.Header().Add(tc.name, value)
				}
				_, _ = fmt.Fprint(w, calls)
			}))
			for i := 1; i <= 2; i++ {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/items", nil))
				if response.Body.String() != fmt.Sprint(i) {
					t.Fatalf("response %d reused response with unrepresentable headers", i)
				}
				got := response.Header().Values(tc.name)
				if len(got) != len(tc.values) {
					t.Fatalf("response %d lost header values: %q", i, got)
				}
				for j := range got {
					if got[j] != tc.values[j] {
						t.Fatalf("response %d changed header: %q want %q", i, got, tc.values)
					}
				}
			}
			if calls != 2 {
				t.Fatalf("unrepresentable response cached: calls=%d", calls)
			}
		})
	}
}
