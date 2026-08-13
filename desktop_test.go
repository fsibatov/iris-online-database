package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesktopAssetOriginIsAccepted(t *testing.T) {
	if !validRequestHost("wails.localhost") {
		t.Fatal("Wails production host was rejected")
	}
	request := httptest.NewRequest(http.MethodGet, "http://wails.localhost/api/meta", nil)
	request.Host = "wails.localhost"
	request.Header.Set("Origin", "http://wails.localhost")
	if !validRequestOrigin(request) {
		t.Fatal("Wails production origin was rejected")
	}
}

func TestExternalURLAllowlist(t *testing.T) {
	allowed := []string{
		"https://github.com/fsibatov/iris-online-database/releases/latest",
		"https://vk.ru/wall-59626511_62336",
		"https://docs.google.com/spreadsheets/d/example/edit#gid=1",
		"https://wiki.irisonline.ru/",
	}
	for _, target := range allowed {
		if got, err := validateExternalURL(target); err != nil || got != target {
			t.Errorf("validateExternalURL(%q) = %q, %v", target, got, err)
		}
	}

	blocked := []string{
		"http://github.com/fsibatov/iris-online-database",
		"https://example.com/",
		"https://github.com.evil.invalid/",
		"https://user:password@github.com/",
		"https://github.com:444/",
		"https://127.0.0.1/",
		" https://github.com/",
		"https://github.com/\nhttps://evil.invalid/",
	}
	for _, target := range blocked {
		if got, err := validateExternalURL(target); err == nil || got != "" {
			t.Errorf("validateExternalURL(%q) unexpectedly accepted %q", target, got)
		}
	}
}

func TestSecurityHeadersProtectWailsStaticAssets(t *testing.T) {
	handler := withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "http://wails.localhost/index.html", nil)
	request.Host = "wails.localhost"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	csp := recorder.Header().Get("Content-Security-Policy")
	for _, required := range []string{"default-src 'self'", "connect-src 'self'", "frame-src 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, required) {
			t.Fatalf("CSP is missing %q: %q", required, csp)
		}
	}
}
