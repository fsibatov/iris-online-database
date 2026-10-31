package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A cached gzip body without Content-Encoding is undecodable by the client.
// Since the cache currently persists only Content-Type, encoded responses
// must bypass it instead of silently losing response metadata.
func TestCacheDoesNotStripContentEncoding(t *testing.T) {
	var calls atomic.Int64
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write([]byte(`{"server":"original"}`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	a := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed.Bytes())
	}))
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/items?server=original", nil))
		if recorder.Code != 200 {
			t.Fatalf("request %d status %d", i, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("request %d lost Content-Encoding: %q", i, got)
		}
		zr, err := gzip.NewReader(bytes.NewReader(recorder.Body.Bytes()))
		if err != nil {
			t.Fatalf("request %d invalid gzip: %v", i, err)
		}
		body, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil || string(body) != `{"server":"original"}` {
			t.Fatalf("request %d decoded %q: %v", i, body, err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("encoded response unexpectedly cached: calls=%d", got)
	}
}

func TestCacheDoesNotStripContentRange(t *testing.T) {
	var calls atomic.Int64
	a := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 0-3/8")
		_, _ = fmt.Fprint(w, calls.Add(1))
	}))
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/items", nil))
		if rec.Header().Get("Content-Range") != "bytes 0-3/8" {
			t.Fatalf("request %d lost Content-Range", i)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("ranged response unexpectedly cached: calls=%d", got)
	}
}

func TestCacheStillStoresPlainJSONAfterHeaderGuard(t *testing.T) {
	var calls atomic.Int64
	a := &application{cache: newResponseCache(32, 1<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/items", nil))
		if rec.Code != 200 || rec.Body.String() != `{"ok":true}` {
			t.Fatalf("request %d failed: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("plain JSON cache regressed: calls=%d", got)
	}
}
