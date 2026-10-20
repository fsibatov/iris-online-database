package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The oversized-body path commits headers inside captureWriter.Write, before
// withResponseCache can perform its post-backend cancellation check. A request
// canceled while the backend is still preparing the stream must not receive
// its buffered prefix or the newly generated body.
func TestStreamingCacheRejectsCancellationBeforeFirstCommit(t *testing.T) {
	app := &application{cache: newResponseCache(8, 256, time.Minute)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	verifyInterruptedStream(t, app, ctx, cancel, http.StatusRequestTimeout)
}

func TestStreamingCacheRejectsShutdownBeforeFirstCommit(t *testing.T) {
	for _, mode := range []string{"closing", "application_context"} {
		t.Run(mode, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 256, time.Minute)}
			if mode == "application_context" {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				app.ctx = ctx
				verifyInterruptedStream(t, app, context.Background(), cancel, http.StatusServiceUnavailable)
				return
			}
			verifyInterruptedStream(t, app, context.Background(), func() {
				app.closing.Store(true)
				app.cache.Close()
			}, http.StatusServiceUnavailable)
		})
	}
}

func verifyInterruptedStream(t *testing.T, app *application, requestContext context.Context, interrupt func(), wantStatus int) {
	t.Helper()
	entered := make(chan struct{})
	continueBackend := make(chan struct{})
	writeErrors := make(chan [2]error, 1)
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("X-Secret", "must-not-escape")
		_, _ = w.Write([]byte("buffered-prefix")) // Under the 128-byte threshold.
		close(entered)
		<-continueBackend
		_, firstErr := w.Write(bytes.Repeat([]byte("S"), 256)) // Forces streaming.
		_, secondErr := w.Write([]byte("late-private-body"))
		writeErrors <- [2]error{firstErr, secondErr}
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(requestContext)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() { defer close(finished); handler.ServeHTTP(rec, req) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(continueBackend)
		t.Fatal("backend did not reach streaming boundary")
	}
	interrupt()
	close(continueBackend)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("interrupted streaming backend did not finish")
	}
	if rec.Code != wantStatus || strings.Contains(rec.Body.String(), "buffered-prefix") || strings.Contains(rec.Body.String(), "late-private-body") || strings.Contains(rec.Body.String(), "SSSS") {
		t.Fatalf("interrupted stream leaked: status=%d body=%q; want %d", rec.Code, rec.Body.String(), wantStatus)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Secret") != "" {
		t.Fatalf("interrupted stream headers not protected: %v", rec.Header())
	}
	if errs := <-writeErrors; errs[0] == nil || errs[1] == nil {
		t.Fatalf("interrupted writes did not report errors: %v", errs)
	}
	app.cache.mu.Lock()
	defer app.cache.mu.Unlock()
	if len(app.cache.entries) != 0 {
		t.Fatalf("interrupted stream was cached: %d entries", len(app.cache.entries))
	}
}
