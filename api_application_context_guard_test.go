package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A canceled application context is a shutdown signal even if the closing
// flag has not yet been set. Uncached API mutations must not reach handlers.
func TestAPIGuardRejectsCanceledApplicationContext(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/health"},
		{http.MethodPut, "/api/user-data"},
		{http.MethodPost, "/api/favorites"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			app := &application{ctx: ctx, cache: newResponseCache(8, 1<<20, time.Minute)}
			calls := 0
			handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = "wails.localhost"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable || calls != 0 {
				t.Fatalf("canceled application: status=%d backend calls=%d, want 503/0", rec.Code, calls)
			}
			if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") == "" {
				t.Fatalf("shutdown response lost security headers: %v", rec.Header())
			}
		})
	}
}

func TestAPIGuardShutdownContextPrecedesRequestCancellation(t *testing.T) {
	ctx, stopApp := context.WithCancel(context.Background())
	stopApp()
	requestCtx, stopRequest := context.WithCancel(context.Background())
	stopRequest()
	app := &application{ctx: ctx, cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPut, "/api/user-data", nil).WithContext(requestCtx)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || calls != 0 {
		t.Fatalf("shutdown precedence: status=%d backend calls=%d, want 503/0", rec.Code, calls)
	}
}

func TestAPIGuardAllowsActiveUncachedRequest(t *testing.T) {
	app := &application{ctx: context.Background(), cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPut, "/api/user-data", nil)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || calls != 1 {
		t.Fatalf("active request: status=%d backend calls=%d, want 204/1", rec.Code, calls)
	}
}
