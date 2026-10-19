package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Rejected requests must receive the same no-store and browser protections
// as successful requests. A forbidden response must not be cacheable.
func TestSecurityHeadersProtectRejectedRequests(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*http.Request)
	}{
		{name: "foreign host", prepare: func(r *http.Request) { r.Host = "untrusted.example" }},
		{name: "cross-site", prepare: func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{name: "foreign origin", prepare: func(r *http.Request) { r.Header.Set("Origin", "https://untrusted.example") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := withSecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			req := httptest.NewRequest(http.MethodPost, "http://wails.localhost/api/user-data", nil)
			tc.prepare(req)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusForbidden || called {
				t.Fatalf("request not rejected: status=%d handlerCalled=%v", response.Code, called)
			}
			for name, want := range map[string]string{
				"Cache-Control":                "no-store",
				"X-Content-Type-Options":       "nosniff",
				"X-Frame-Options":              "DENY",
				"Cross-Origin-Resource-Policy": "same-origin",
				"Referrer-Policy":              "no-referrer",
			} {
				if got := response.Header().Get(name); got != want {
					t.Errorf("%s=%q, want %q", name, got, want)
				}
			}
			if response.Header().Get("Content-Security-Policy") == "" {
				t.Error("rejected response has no CSP")
			}
		})
	}
}
