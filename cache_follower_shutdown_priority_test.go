package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Signal when the follower has reached the cancellation select, so the test
// does not depend on scheduler timing or arbitrary sleeps.
type cacheFollowerSignaledContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *cacheFollowerSignaledContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestCacheFollowerShutdownTakesPriorityOverRequestCancellation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		shutdown     bool
		shutdownCtx  bool
		wantStatus   int
		wantFragment string
	}{
		{name: "shutdown_and_cancel", shutdown: true, wantStatus: http.StatusServiceUnavailable, wantFragment: "завершает работу"},
		{name: "shutdown_context_and_cancel", shutdownCtx: true, wantStatus: http.StatusServiceUnavailable, wantFragment: "завершает работу"},
		{name: "cancel_only", wantStatus: http.StatusRequestTimeout, wantFragment: "отменён"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{cache: newResponseCache(8, 1<<20, time.Minute)}
			if tc.shutdownCtx {
				appCtx, stopApp := context.WithCancel(context.Background())
				app.ctx = appCtx
				defer stopApp()
				app.cancel = stopApp
			}
			var calls atomic.Int64
			entered := make(chan struct{})
			release := make(chan struct{})
			leaderDone := make(chan struct{})
			handler := app.withAPIRequestPipeline(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					close(entered)
				}
				<-release
				_, _ = w.Write([]byte("catalog body"))
			}))
			go func() {
				defer close(leaderDone)
				request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
				request.Host = "wails.localhost"
				handler.ServeHTTP(httptest.NewRecorder(), request)
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("cache leader did not reach backend")
			}

			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &cacheFollowerSignaledContext{Context: base, waiting: make(chan struct{})}
			request := httptest.NewRequest(http.MethodGet, "/api/items", nil).WithContext(ctx)
			request.Host = "wails.localhost"
			response := httptest.NewRecorder()
			followerDone := make(chan struct{})
			go func() { defer close(followerDone); handler.ServeHTTP(response, request) }()
			select {
			case <-ctx.waiting:
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("cache follower did not enter cancellation wait")
			}
			// Shutdown may begin before the follower's request context is
			// canceled. Its 503 must not be replaced by the 408 select arm.
			if tc.shutdown {
				app.closing.Store(true)
			}
			if tc.shutdownCtx {
				app.cancel()
			}
			cancel()
			select {
			case <-followerDone:
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("cache follower remained blocked")
			}
			close(release)
			select {
			case <-leaderDone:
			case <-time.After(2 * time.Second):
				t.Fatal("cache leader remained blocked")
			}
			if response.Code != tc.wantStatus || !strings.Contains(response.Body.String(), tc.wantFragment) {
				t.Fatalf("status=%d body=%q; want %d with %q", response.Code, response.Body.String(), tc.wantStatus, tc.wantFragment)
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Security-Policy") == "" {
				t.Fatalf("security headers missing: %v", response.Header())
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("follower invoked backend: calls=%d", got)
			}
		})
	}
}
