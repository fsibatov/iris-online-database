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

// When a leader cannot cache its response, all waiting followers must retry
// ownership instead of bypassing the in-flight gate and flooding the backend.
func TestPipelineUncacheableBurstRetainsSingleflight(t *testing.T) {
	var calls atomic.Int64
	var active atomic.Int64
	var peak atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		for {
			prev := peak.Load()
			if current <= prev || peak.CompareAndSwap(prev, current) {
				break
			}
		}
		defer active.Add(-1)
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		time.Sleep(2 * time.Millisecond)
		w.Header().Set("Cache-Control", "private")
		_, _ = fmt.Fprint(w, `{"private":true}`)
	}))
	const requests = 64
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
			if rec.Code != 200 || rec.Body.String() != `{"private":true}` {
				results <- fmt.Sprintf("status=%d body=%q", rec.Code, rec.Body.String())
			}
		}()
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("leader not started")
	}
	time.Sleep(80 * time.Millisecond)
	close(release)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("requests deadlocked")
	}
	close(results)
	for result := range results {
		t.Error(result)
	}
	if got := calls.Load(); got != requests {
		t.Errorf("backend calls=%d, want %d", got, requests)
	}
	if got := peak.Load(); got != 1 {
		t.Errorf("peak concurrent backend calls=%d, want 1", got)
	}
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()
	if got := len(a.cache.inflight); got != 0 {
		t.Errorf("inflight leak: %d", got)
	}
}
