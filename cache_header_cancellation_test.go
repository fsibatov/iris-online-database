package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// WriteHeader is an irreversible boundary. If cancellation starts while the
// destination commits headers, neither a buffered miss nor a cached hit may
// subsequently send the response body.
type cancelAtHeaderCommit struct {
	*httptest.ResponseRecorder
	interrupt func()
}

func (w *cancelAtHeaderCommit) WriteHeader(status int) {
	w.ResponseRecorder.WriteHeader(status)
	w.interrupt()
}

func testHeaderCommitCancellation(t *testing.T, cacheHit bool, mode string) {
	t.Helper()
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	defer cancelRequest()
	interrupt := cancelRequest
	if mode == "application_context" {
		appCtx, cancelApp := context.WithCancel(context.Background())
		defer cancelApp()
		app.ctx = appCtx
		interrupt = cancelApp
	} else if mode == "closing" {
		interrupt = func() { app.closing.Store(true) }
	}

	calls := 0
	const body = `{"catalog":"public"}`
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte(body))
	}))
	request := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil)
		req.Host = "wails.localhost"
		return req
	}
	if cacheHit {
		seed := httptest.NewRecorder()
		handler.ServeHTTP(seed, request())
		if seed.Code != http.StatusOK || seed.Body.String() != body || calls != 1 {
			t.Fatalf("could not seed cache: status=%d body=%q calls=%d", seed.Code, seed.Body.String(), calls)
		}
	}

	canceled := request().WithContext(requestCtx)
	recorder := &cancelAtHeaderCommit{ResponseRecorder: httptest.NewRecorder(), interrupt: interrupt}
	handler.ServeHTTP(recorder, canceled)
	if recorder.Code != http.StatusOK {
		t.Errorf("already committed status=%d, want 200", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("private body written after header-commit cancellation: %q", recorder.Body.String())
	}
	if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("committed response headers lost: %v", recorder.Header())
	}
	if cacheHit {
		if calls != 1 {
			t.Errorf("cached response called backend: calls=%d", calls)
		}
		if _, ok := app.cache.Get(cacheKeyForRequest(request())); !ok {
			t.Error("existing healthy cache entry was removed")
		}
	} else {
		if calls != 1 {
			t.Errorf("buffered miss backend calls=%d, want 1", calls)
		}
		if _, ok := app.cache.Get(cacheKeyForRequest(request())); ok {
			t.Error("response canceled at header commit populated cache")
		}
	}

	if mode == "request" {
		healthy := httptest.NewRecorder()
		handler.ServeHTTP(healthy, request())
		if healthy.Code != http.StatusOK || healthy.Body.String() != body {
			t.Errorf("healthy retry: status=%d body=%q", healthy.Code, healthy.Body.String())
		}
		wantCalls := 2
		if cacheHit {
			wantCalls = 1
		}
		if calls != wantCalls {
			t.Errorf("backend calls after healthy retry=%d, want %d", calls, wantCalls)
		}
	}
}

func TestBufferedMissStopsBodyAtHeaderCommitCancellation(t *testing.T) {
	for _, mode := range []string{"request", "application_context", "closing"} {
		t.Run(mode, func(t *testing.T) { testHeaderCommitCancellation(t, false, mode) })
	}
}

func TestCachedHitStopsBodyAtHeaderCommitCancellation(t *testing.T) {
	for _, mode := range []string{"request", "application_context", "closing"} {
		t.Run(mode, func(t *testing.T) { testHeaderCommitCancellation(t, true, mode) })
	}
}
