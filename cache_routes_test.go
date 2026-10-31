package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheableGameDataRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/titles", "/api/titles/42", "/api/transformations",
		"/api/transformations/3001", "/api/world-source-monsters",
	} {
		t.Run(path, func(t *testing.T) {
			var calls atomic.Int64
			app := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"server":%q,"path":%q}`, r.URL.Query().Get("server"), r.URL.Path)
			}))
			request := func(server string) string {
				req := httptest.NewRequest(http.MethodGet, path+"?server="+server, nil)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, req)
				if recorder.Code != http.StatusOK {
					t.Fatalf("unexpected status: %d", recorder.Code)
				}
				if recorder.Header().Get("Content-Type") != "application/json" {
					t.Fatal("content type lost")
				}
				return recorder.Body.String()
			}
			original := request("original")
			if second := request("original"); second != original {
				t.Fatal("same-server cache miss or corruption")
			}
			kiss := request("kiss")
			if original == kiss {
				t.Fatal("cross-server data contamination")
			}
			if second := request("kiss"); second != kiss {
				t.Fatal("kiss cache miss or corruption")
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("expected one backend request per server, got %d", got)
			}
		})
	}
}

func TestResponseCacheNeverStoresPrivateOrDynamicRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/user-data", "/api/favorites", "/api/update-check", "/api/community-status", "/api/health",
	} {
		t.Run(path, func(t *testing.T) {
			var calls atomic.Int64
			app := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
			handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, calls.Add(1))
			}))
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if rec.Body.String() != fmt.Sprint(i+1) {
					t.Fatalf("route was cached: %q", rec.Body.String())
				}
			}
		})
	}
	if isCacheableResponseRequest(httptest.NewRequest(http.MethodPost, "/api/titles", strings.NewReader("{}"))) {
		t.Fatal("POST must not be cached")
	}
}
