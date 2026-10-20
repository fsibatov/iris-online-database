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

// Once streaming headers are committed, cancellation cannot replace HTTP 200
// with an error status. It must nevertheless stop subsequent body writes:
// a canceled request or closing application must not keep delivering data.
func TestStreamingStopsBodyWritesAfterCommittedCancellation(t *testing.T) {
	for _, mode := range []string{"request", "application_context", "closing"} {
		t.Run(mode, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 128, time.Minute)}
			reqCtx, cancelRequest := context.WithCancel(context.Background())
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

			firstSent := make(chan struct{})
			resume := make(chan struct{})
			type writes struct {
				firstN, secondN, thirdN       int
				firstErr, secondErr, thirdErr error
			}
			results := make(chan writes, 1)
			handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Cache-Control", "public, max-age=60")
				firstN, firstErr := w.Write([]byte(strings.Repeat("A", 96))) // 64-byte cache threshold.
				close(firstSent)
				<-resume
				secondN, secondErr := w.Write([]byte("PRIVATE-CONTINUATION"))
				thirdN, thirdErr := w.Write([]byte("AFTER-CANCELLATION"))
				results <- writes{firstN, secondN, thirdN, firstErr, secondErr, thirdErr}
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(reqCtx)
			req.Host = "wails.localhost"
			recorder := httptest.NewRecorder()
			finished := make(chan struct{})
			go func() { defer close(finished); handler.ServeHTTP(recorder, req) }()
			select {
			case <-firstSent:
			case <-time.After(2 * time.Second):
				close(resume)
				t.Fatal("stream did not commit its first chunk")
			}
			interrupt()
			close(resume)
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("canceled streaming backend did not finish")
			}
			got := <-results
			if got.firstN != 96 || got.firstErr != nil || got.secondN != 0 || got.thirdN != 0 || !errors.Is(got.secondErr, context.Canceled) || !errors.Is(got.thirdErr, context.Canceled) {
				t.Errorf("writes after commit/cancellation: %+v; want first=96/nil, subsequent=0/context.Canceled", got)
			}
			if recorder.Code != http.StatusOK || recorder.Body.String() != strings.Repeat("A", 96) {
				t.Errorf("committed stream continued after cancellation: status=%d body=%q", recorder.Code, recorder.Body.String())
			}
			if recorder.Header().Get("Content-Security-Policy") == "" || recorder.Header().Get("Cache-Control") != "public, max-age=60" {
				t.Errorf("committed response lost security/cache headers: %v", recorder.Header())
			}
			if _, ok := app.cache.Get(cacheKeyForRequest(req)); ok {
				t.Error("interrupted stream populated response cache")
			}
		})
	}
}

func TestStreamingUninterruptedContinuationStillWorks(t *testing.T) {
	app := &application{cache: newResponseCache(8, 128, time.Minute)}
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if n, err := w.Write([]byte(strings.Repeat("A", 96))); n != 96 || err != nil {
			t.Errorf("first chunk: n=%d err=%v", n, err)
		}
		if n, err := w.Write([]byte("CONTINUATION")); n != len("CONTINUATION") || err != nil {
			t.Errorf("second chunk: n=%d err=%v", n, err)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != strings.Repeat("A", 96)+"CONTINUATION" {
		t.Fatalf("active stream: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
