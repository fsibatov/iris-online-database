package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A request canceled before admission must never occupy an available
// execution slot or invoke a backend. Both select cases are otherwise ready
// and Go may randomly choose the backend path even for a canceled context.
func TestAPIConcurrencyLimitRejectsAlreadyCanceledRequests(t *testing.T) {
	calls := 0
	handler := withAPIConcurrencyLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}), 1)

	for i := 0; i < 64; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		request := httptest.NewRequest(http.MethodGet, "/api/health", nil).WithContext(ctx)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusRequestTimeout {
			t.Fatalf("canceled request %d: status=%d, want %d", i, response.Code, http.StatusRequestTimeout)
		}
	}
	if calls != 0 {
		t.Fatalf("canceled requests invoked backend %d times", calls)
	}
}

func TestAPIConcurrencyLimitStillAdmitsActiveRequest(t *testing.T) {
	calls := 0
	handler := withAPIConcurrencyLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}), 1)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusNoContent || calls != 1 {
		t.Fatalf("active request: status=%d backend calls=%d", response.Code, calls)
	}
}
