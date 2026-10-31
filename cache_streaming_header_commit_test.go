package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Streaming a buffered prefix crosses the irreversible WriteHeader boundary.
// A shutdown or cancellation triggered by that call must not release the
// previously buffered bytes before the next lifecycle check.
func TestStreamingPrefixStopsAtHeaderCommitCancellation(t *testing.T) {
	for _, mode := range []string{"request", "application_context", "closing"} {
		t.Run(mode, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 64, time.Minute)}
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
			var firstN, overflowN, laterN int
			var firstErr, overflowErr, laterErr error
			handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Cache-Control", "public, max-age=60")
				firstN, firstErr = w.Write([]byte("PRIVATE-PREFIX"))
				overflowN, overflowErr = w.Write([]byte(strings.Repeat("S", 48)))
				laterN, laterErr = w.Write([]byte("AFTER-CANCEL"))
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(requestCtx)
			req.Host = "wails.localhost"
			recorder := &cancelAtHeaderCommit{ResponseRecorder: httptest.NewRecorder(), interrupt: interrupt}
			handler.ServeHTTP(recorder, req)

			if calls != 1 || recorder.Code != http.StatusOK {
				t.Errorf("backend calls=%d committed status=%d, want 1/200", calls, recorder.Code)
			}
			if recorder.Body.Len() != 0 {
				t.Errorf("buffered prefix leaked after WriteHeader cancellation: %q", recorder.Body.String())
			}
			if firstN != len("PRIVATE-PREFIX") || firstErr != nil || overflowN != 0 || laterN != 0 || !errors.Is(overflowErr, context.Canceled) || !errors.Is(laterErr, context.Canceled) {
				t.Errorf("writes first=(%d,%v), overflow=(%d,%v), later=(%d,%v)", firstN, firstErr, overflowN, overflowErr, laterN, laterErr)
			}
			if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("Cache-Control") != "public, max-age=60" {
				t.Errorf("security/cache headers lost: %v", recorder.Header())
			}
			if _, ok := app.cache.Get(cacheKeyForRequest(req)); ok {
				t.Error("canceled streaming response populated cache")
			}
		})
	}
}

func TestStreamingPrefixHealthyAtHeaderCommit(t *testing.T) {
	app := &application{cache: newResponseCache(8, 64, time.Minute)}
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if n, err := w.Write([]byte("PUBLIC-PREFIX")); n != len("PUBLIC-PREFIX") || err != nil {
			t.Errorf("buffered prefix n=%d err=%v", n, err)
		}
		if n, err := w.Write([]byte(strings.Repeat("S", 48))); n != 48 || err != nil {
			t.Errorf("stream transition n=%d err=%v", n, err)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.Host = "wails.localhost"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "PUBLIC-PREFIX"+strings.Repeat("S", 48) {
		t.Errorf("healthy stream status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
