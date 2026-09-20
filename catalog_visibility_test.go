package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHiddenItemsAreExcludedFromCatalogAndSearch(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	var hidden []*Item
	for i := range store.data.Items {
		if store.data.Items[i].Subcategory == "---------" {
			hidden = append(hidden, &store.data.Items[i])
		}
	}
	if len(hidden) != 16 {
		t.Fatalf("test-category fixture changed: %d items", len(hidden))
	}
	for _, server := range []string{"original", "kiss"} {
		t.Run(server, func(t *testing.T) {
			for _, item := range hidden {
				for _, query := range []string{fmt.Sprint(item.ID), item.Name} {
					for _, endpoint := range []string{"/api/items", "/api/search"} {
						values := url.Values{"q": {query}, "server": {server}}
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodGet, endpoint+"?"+values.Encode(), nil)
						if endpoint == "/api/items" {
							handleItems(rec, req)
						} else {
							handleSearch(rec, req)
						}
						if rec.Code != http.StatusOK {
							t.Fatalf("%s returned %d", endpoint, rec.Code)
						}
						var response struct {
							Items []Item `json:"items"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
							t.Fatal(err)
						}
						for _, result := range response.Items {
							if result.Subcategory == "---------" {
								t.Fatalf("test item %d leaked through %s", result.ID, endpoint)
							}
						}
					}
				}
			}
			rec := httptest.NewRecorder()
			values := url.Values{"category": {"Расходники"}, "server": {server}}
			handleItems(rec, httptest.NewRequest(http.MethodGet, "/api/items?"+values.Encode(), nil))
			var response struct {
				Total   int `json:"total"`
				Filters struct {
					Subcategories []string `json:"subcategories"`
				} `json:"filters"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Total < 1 || len(response.Filters.Subcategories) < 1 {
				t.Fatal("normal consumables must remain available")
			}
			for _, category := range response.Filters.Subcategories {
				if category == "---------" {
					t.Fatal("test subcategory leaked into filters")
				}
			}
		})
	}
}

func TestHiddenItemsPreserveSourceAndFavoriteKeys(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	const id = 1200024
	item := store.itemsByID[id]
	if item == nil || item.Subcategory != "---------" {
		t.Fatal("original record must be preserved")
	}
	rec := httptest.NewRecorder()
	handleItem(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d", id), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden item detail returned %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"keys":["item:1200024"],"server":"kiss"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	handleFavorites(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("favorites returned %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Rows    []any `json:"rows"`
		Missing int   `json:"missing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 0 || response.Missing != 1 {
		t.Fatalf("hidden favorite must be unavailable: %+v", response)
	}
	if store.itemsByID[id] != item {
		t.Fatal("presentation filtering changed source storage")
	}
}
