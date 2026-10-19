package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCaptureWriterStreamsOversizedBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := newStreamingCaptureWriter(recorder, 8)
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Add("X-Test", "first")
	writer.Header().Add("X-Test", "second")
	writer.WriteHeader(http.StatusAccepted)
	if _, err := writer.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if writer.streaming || writer.body.Len() != 5 {
		t.Fatal("small initial chunk should remain buffered")
	}
	if _, err := writer.Write([]byte("67890")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("extra")); err != nil {
		t.Fatal(err)
	}
	if !writer.streaming || writer.body.Len() != 0 {
		t.Fatal("oversized body must stream without retaining the buffer")
	}
	if recorder.Code != http.StatusAccepted || recorder.Body.String() != "1234567890extra" {
		t.Fatalf("streamed response corrupted: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "application/octet-stream" || len(recorder.Header().Values("X-Test")) != 2 {
		t.Fatalf("response headers lost: %v", recorder.Header())
	}
}

func TestResponseCacheStreamsLargeResponsesWithoutCaching(t *testing.T) {
	var calls atomic.Int64
	payload := bytes.Repeat([]byte("x"), 4096)
	app := &application{cache: newResponseCache(4, 1024, time.Minute)}
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		for start := 0; start < len(payload); start += 100 {
			end := start + 100
			if end > len(payload) {
				end = len(payload)
			}
			_, _ = w.Write(payload[start:end])
		}
	}))
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil))
		if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), payload) {
			t.Fatalf("response %d corrupted", i)
		}
		if recorder.Header().Get("Content-Type") != "application/json" {
			t.Fatal("content type lost")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("oversized responses must not be cached; backend calls=%d", calls.Load())
	}
}

func TestResponseCacheStillCachesSmallResponses(t *testing.T) {
	var calls atomic.Int64
	app := &application{cache: newResponseCache(4, 1024, time.Minute)}
	handler := app.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil))
		if recorder.Body.String() != `{"ok":true}` {
			t.Fatal("response corrupted")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("small response not cached: calls=%d", calls.Load())
	}
}
