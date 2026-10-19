package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentMissesShareOneCatalogResponse(t *testing.T) {
	var calls atomic.Int64
	gate := make(chan struct{})
	started := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-gate
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"server":"original"}`)
	}))
	const n = 48
	var wg sync.WaitGroup
	errors := make(chan string, n)
	ready := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/items?server=original", nil))
			if recorder.Code != 200 || recorder.Body.String() != `{"server":"original"}` {
				errors <- fmt.Sprintf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		}()
	}
	close(ready)
	<-started
	// The producer is blocked; all followers must be able to join it.
	time.Sleep(40 * time.Millisecond)
	close(gate)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 backend call for %d concurrent requests; got %d", n, calls.Load())
	}
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()
	if len(a.cache.inflight) != 0 {
		t.Fatalf("inflight entries leaked: %d", len(a.cache.inflight))
	}
}

func TestConcurrentMissesRemainServerIsolated(t *testing.T) {
	var calls atomic.Int64
	enteredOriginal := make(chan struct{})
	releaseOriginal := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("server") == "original" {
			close(enteredOriginal)
			<-releaseOriginal
		}
		_, _ = fmt.Fprint(w, r.URL.Query().Get("server"))
	}))
	doneOriginal := make(chan struct{})
	go func() {
		defer close(doneOriginal)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/items?server=original", nil))
	}()
	<-enteredOriginal
	kiss := httptest.NewRecorder()
	handler.ServeHTTP(kiss, httptest.NewRequest("GET", "/api/items?server=kiss", nil))
	if kiss.Body.String() != "kiss" {
		t.Fatalf("other server request blocked or corrupted: %q", kiss.Body.String())
	}
	close(releaseOriginal)
	<-doneOriginal
	if calls.Load() != 2 {
		t.Fatalf("expected separate backend calls for servers; got %d", calls.Load())
	}
}

func TestCanceledFollowerDoesNotRunBackend(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(entered)
		<-release
		_, _ = fmt.Fprint(w, "ok")
	}))
	leaderDone := make(chan struct{})
	go func() {
		defer close(leaderDone)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/items", nil))
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	follower := httptest.NewRecorder()
	followerDone := make(chan struct{})
	go func() {
		defer close(followerDone)
		handler.ServeHTTP(follower, httptest.NewRequest("GET", "/api/items", nil).WithContext(ctx))
	}()
	cancel()
	select {
	case <-followerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled follower blocked")
	}
	if follower.Code != http.StatusRequestTimeout {
		t.Fatalf("canceled follower status %d", follower.Code)
	}
	close(release)
	<-leaderDone
	if calls.Load() != 1 {
		t.Fatalf("canceled follower unexpectedly ran backend: %d", calls.Load())
	}
}

func TestUncacheableResponseDoesNotStrandWaiters(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		w.Header().Set("Cache-Control", "private")
		_, _ = fmt.Fprint(w, "private")
	}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/items", nil))
	}()
	<-entered
	followerDone := make(chan struct{})
	go func() {
		defer close(followerDone)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/items", nil))
	}()
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("leader blocked")
	}
	select {
	case <-followerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("follower blocked")
	}
	if calls.Load() != 2 {
		t.Fatalf("private response was cached: calls=%d", calls.Load())
	}
}

func TestCacheBypassesNoCacheRequestHeaders(t *testing.T) {
	for _, header := range []struct{ name, value string }{{"Cache-Control", "no-cache"}, {"Cache-Control", "MAX-AGE=\"0\""}, {"Pragma", "no-cache"}} {
		t.Run(header.name+"_"+header.value, func(t *testing.T) {
			var calls atomic.Int64
			a := &application{cache: newResponseCache(128, 8<<20, time.Minute)}
			handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, calls.Add(1)) }))
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest("GET", "/api/items", nil)
				req.Header.Set(header.name, header.value)
				handler.ServeHTTP(httptest.NewRecorder(), req)
			}
			if calls.Load() != 2 {
				t.Fatalf("cache ignored request no-cache: calls=%d", calls.Load())
			}
		})
	}
}
