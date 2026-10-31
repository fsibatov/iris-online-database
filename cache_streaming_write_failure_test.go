package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Simulate a destination that reports a short or failed Write after the
// response has switched from buffering to streaming. A stream cannot be
// repaired after its headers are committed, but the backend must see the
// failure and must not append a second, misleading body fragment.
type failingStreamDestination struct {
	*httptest.ResponseRecorder
	failCall int
	limit    int
	writeErr error
	calls    int
}

func (w *failingStreamDestination) Write(data []byte) (int, error) {
	w.calls++
	n := len(data)
	if w.calls == w.failCall {
		if n > w.limit {
			n = w.limit
		}
		if n > 0 {
			_, _ = w.ResponseRecorder.Write(data[:n])
		}
		return n, w.writeErr
	}
	return w.ResponseRecorder.Write(data)
}

func testStreamingDestinationFailure(t *testing.T, failCall, limit int, injected error, wantFirstBytes int) {
	t.Helper()
	app := &application{cache: newResponseCache(8, 64, time.Minute)}
	var secondN int
	var secondErr, thirdErr error
	var thirdN int
	backendCalls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls++
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "public, max-age=60")
		if n, err := w.Write([]byte("prefix")); n != 6 || err != nil {
			t.Errorf("buffering prefix: n=%d err=%v", n, err)
		}
		secondN, secondErr = w.Write([]byte(strings.Repeat("B", 48)))
		thirdN, thirdErr = w.Write([]byte("AFTER-FAILURE"))
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	r.Host = "wails.localhost"
	destination := &failingStreamDestination{
		ResponseRecorder: httptest.NewRecorder(),
		failCall:         failCall,
		limit:            limit,
		writeErr:         injected,
	}
	handler.ServeHTTP(destination, r)
	if destination.Code != http.StatusOK || backendCalls != 1 {
		t.Fatalf("status=%d backend calls=%d, want 200/1", destination.Code, backendCalls)
	}
	if secondN != wantFirstBytes || secondErr == nil || thirdN != 0 || thirdErr == nil {
		t.Errorf("stream write results: second=(%d,%v) third=(%d,%v), want second bytes=%d and both errors", secondN, secondErr, thirdN, thirdErr, wantFirstBytes)
	}
	if injected == nil && !errors.Is(secondErr, io.ErrShortWrite) {
		t.Errorf("short destination Write without error: got %v, want io.ErrShortWrite", secondErr)
	}
	if strings.Contains(destination.Body.String(), "AFTER-FAILURE") {
		t.Errorf("backend appended data after failed stream: %q", destination.Body.String())
	}
	if destination.Header().Get("Content-Security-Policy") == "" || destination.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("stream lost security or cache policy: %v", destination.Header())
	}
	if _, ok := app.cache.Get(cacheKeyForRequest(r)); ok {
		t.Error("failed stream entered response cache")
	}
}

func TestStreamingBufferedPrefixWriteFailureStopsStream(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		err   error
	}{
		{"short-without-error", 2, nil},
		{"zero-with-error", 0, io.ErrClosedPipe},
		{"full-with-error", 6, io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testStreamingDestinationFailure(t, 1, tc.limit, tc.err, 0)
		})
	}
}

func TestStreamingCurrentChunkShortWriteStopsStream(t *testing.T) {
	// The prefix is delivered successfully. The oversized second chunk is
	// short-written without an error; subsequent writes must not continue.
	testStreamingDestinationFailure(t, 2, 3, nil, 3)
}
