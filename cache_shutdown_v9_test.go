package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The application's context is canceled during shutdown. A follower waiting
// for a slow response must not remain blocked until the producer finishes.
func TestCacheFollowerStopsPromptlyOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &application{ctx: ctx, cancel: cancel, cache: newResponseCache(32, 1<<20, time.Minute)}
	var calls atomic.Int64
	started := make(chan struct{})
	release := make(chan struct{})
	handler := a.withResponseCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		_, _ = fmt.Fprint(w, "ok")
	}))
	leaderDone := make(chan struct{})
	go func() {
		defer close(leaderDone)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/items", nil))
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("leader did not start")
	}
	follower := httptest.NewRecorder()
	followerDone := make(chan struct{})
	go func() {
		defer close(followerDone)
		handler.ServeHTTP(follower, httptest.NewRequest("GET", "/api/items", nil))
	}()
	// The producer is intentionally held; shutdown must release the follower
	// without waiting for the backend to finish.
	a.requestShutdown("test")
	select {
	case <-followerDone:
	case <-time.After(time.Second):
		close(release)
		<-leaderDone
		t.Fatal("follower remained blocked after shutdown")
	}
	if follower.Code != http.StatusServiceUnavailable {
		t.Errorf("follower status=%d, want 503", follower.Code)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("backend calls after shutdown=%d, want 1", got)
	}
	close(release)
	select {
	case <-leaderDone:
	case <-time.After(time.Second):
		t.Fatal("leader did not finish")
	}
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()
	if got := len(a.cache.inflight); got != 0 {
		t.Fatalf("inflight entries leaked: %d", got)
	}
}
