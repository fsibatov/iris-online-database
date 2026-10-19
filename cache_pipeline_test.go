package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// This exercises the production middleware order, rather than the cache
// middleware in isolation. Waiting cache followers must not exhaust the
// 32-slot backend limiter.
func TestPipelineCoalescesBurstWithout503(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"server":"original"}`)
	}))

	const requests = 96
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan string, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil)
			req.Host = "wails.localhost"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != `{"server":"original"}` {
				results <- fmt.Sprintf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		}()
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("backend did not start")
	}
	// Hold the first request long enough for the other goroutines to join it.
	time.Sleep(75 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	for result := range results {
		t.Error(result)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("want one backend call for %d requests, got %d", requests, got)
	}
}

// Distinct cache keys must still be subject to the original backend limit.
func TestPipelineRetainsBackendConcurrencyLimit(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		_, _ = fmt.Fprint(w, "ok")
	}))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items?id=%d", i), nil)
			req.Host = "wails.localhost"
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	for i := 0; i < 32; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("failed to saturate backend limit")
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items?id=overflow", nil)
	req.Host = "wails.localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	close(release)
	wg.Wait()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("backend limit bypassed: status=%d", rec.Code)
	}
	if calls.Load() != 32 {
		t.Fatalf("expected 32 backend calls, got %d", calls.Load())
	}
}
