package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// observedErrContext signals only after the lifecycle guard has observed a
// live context, allowing cancellation while a subsequent cache lookup waits.
type observedErrContext struct {
	context.Context
	once    sync.Once
	checked chan struct{}
}

func (c *observedErrContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

func TestCachedResponseRechecksCancellationAfterLookup(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprint(w, "cached-catalog")
	}))
	warm := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	warm.Host = "wails.localhost"
	handler.ServeHTTP(httptest.NewRecorder(), warm)
	if calls != 1 {
		t.Fatalf("cache warm-up failed: calls=%d", calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &observedErrContext{Context: ctx, checked: make(chan struct{})}
	request := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(observed)
	request.Host = "wails.localhost"
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	app.cache.mu.Lock()
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()
	select {
	case <-observed.checked:
	case <-time.After(2 * time.Second):
		app.cache.mu.Unlock()
		t.Fatal("request did not pass initial lifecycle check")
	}
	cancel() // After guard, while Get is blocked on the cache mutex.
	app.cache.mu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled cache lookup did not finish")
	}
	if recorder.Code != http.StatusRequestTimeout || recorder.Body.String() == "cached-catalog" {
		t.Fatalf("canceled request received cached response: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if calls != 1 {
		t.Fatalf("cache replay unexpectedly called backend: %d", calls)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("rejected replay lost security headers: %v", recorder.Header())
	}
}

func TestCachedResponseRechecksShutdownAfterLookup(t *testing.T) {
	app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
	calls := 0
	handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprint(w, "cached-catalog")
	}))
	warm := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	warm.Host = "wails.localhost"
	handler.ServeHTTP(httptest.NewRecorder(), warm)
	if calls != 1 {
		t.Fatalf("cache warm-up failed: calls=%d", calls)
	}

	observed := &observedErrContext{Context: context.Background(), checked: make(chan struct{})}
	request := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(observed)
	request.Host = "wails.localhost"
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	app.cache.mu.Lock()
	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()
	select {
	case <-observed.checked:
	case <-time.After(2 * time.Second):
		app.cache.mu.Unlock()
		t.Fatal("request did not pass initial lifecycle check")
	}
	app.closing.Store(true) // Shutdown races with an already admitted cache hit.
	app.cache.mu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cache lookup did not finish during shutdown")
	}
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.String() == "cached-catalog" {
		t.Fatalf("shutdown request received cached response: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if calls != 1 {
		t.Fatalf("cache replay unexpectedly called backend: %d", calls)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("rejected replay lost security headers: %v", recorder.Header())
	}
}
