package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A failed destination Write means the buffered response was not delivered.
// It must not become a shared cache hit for the next, healthy request.
type incompleteResponseWriter struct {
	*httptest.ResponseRecorder
	limit    int
	writeErr error
}

func (w *incompleteResponseWriter) Write(data []byte) (int, error) {
	n := len(data)
	if n > w.limit {
		n = w.limit
	}
	if n > 0 {
		_, _ = w.ResponseRecorder.Write(data[:n])
	}
	return n, w.writeErr
}

func testIncompleteBufferedResponse(t *testing.T, limit int, writeErr error) {
	t.Helper()
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte(`{"catalog":"public"}`))
	}))
	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil)
		r.Host = "wails.localhost"
		return r
	}

	failed := &incompleteResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		limit:            limit,
		writeErr:         writeErr,
	}
	first := request()
	handler.ServeHTTP(failed, first)
	if failed.Code != http.StatusOK || calls != 1 {
		t.Fatalf("failed commit: status=%d backend calls=%d, want 200/1", failed.Code, calls)
	}
	if failed.Header().Get("Content-Security-Policy") == "" || failed.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("failed commit lost response headers: %v", failed.Header())
	}
	if _, ok := app.cache.Get(cacheKeyForRequest(first)); ok {
		t.Fatal("incompletely written response was cached")
	}

	// A retry must execute the backend again; only the successful response may
	// seed the cache, and the next request should reuse that entry.
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request())
		if recorder.Code != http.StatusOK || recorder.Body.String() != `{"catalog":"public"}` {
			t.Fatalf("retry %d: status=%d body=%q", i+1, recorder.Code, recorder.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("backend calls=%d, want 2 (failed commit + successful retry)", calls)
	}
}

func TestBufferedWriteErrorDoesNotPopulateCache(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		err   error
	}{
		{"no-bytes", 0, io.ErrClosedPipe},
		{"partial-body", 4, io.ErrUnexpectedEOF},
		{"full-body-with-error", 1 << 20, errors.New("destination failed after writing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testIncompleteBufferedResponse(t, tc.limit, tc.err)
		})
	}
}

func TestBufferedShortWriteWithoutErrorDoesNotPopulateCache(t *testing.T) {
	// Defend against a broken ResponseWriter that violates io.Writer's
	// short-write contract by reporting n < len(body) without an error.
	testIncompleteBufferedResponse(t, 3, nil)
}
