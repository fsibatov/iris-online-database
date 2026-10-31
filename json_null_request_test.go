package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// JSON null decodes successfully into a Go struct without populating it.
// On /api/user-data this would pass an empty profile to Replace, potentially
// overwriting the user's saved preferences. The endpoint requires an object.
func TestJSONDecoderRejectsNullProfile(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"plain", "null"},
		{"whitespace", " \r\nnull\t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json; charset=utf-8")
			recorder := httptest.NewRecorder()
			var profile struct {
				Favorites []string `json:"favorites"`
			}
			if decodeJSONRequest(recorder, request, &profile, 4096) {
				t.Fatal("JSON null accepted as a replacement profile")
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, want HTTP 400", recorder.Code)
			}
		})
	}
}

// The same decoder is used by the favorites API. Null must not silently
// become a zero-value request, regardless of the concrete object target.
func TestJSONDecoderRejectsNullFavorites(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/favorites", strings.NewReader("null"))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	var payload struct {
		Keys []string `json:"keys"`
	}
	if decodeJSONRequest(recorder, request, &payload, 4096) {
		t.Fatal("JSON null accepted as an empty favorites request")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want HTTP 400", recorder.Code)
	}
}

func TestJSONDecoderPreservesObjectAndNullableField(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/api/user-data", strings.NewReader(`{"favorites":["item:1"],"note":null}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	var payload struct {
		Favorites []string `json:"favorites"`
		Note      *string  `json:"note"`
	}
	if !decodeJSONRequest(recorder, request, &payload, 4096) {
		t.Fatalf("valid JSON object rejected: status=%d", recorder.Code)
	}
	if len(payload.Favorites) != 1 || payload.Favorites[0] != "item:1" || payload.Note != nil {
		t.Fatalf("decoded object changed: %+v", payload)
	}
}
