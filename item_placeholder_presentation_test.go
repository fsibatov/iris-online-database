package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPlaceholderSubcategoryPresentation(t *testing.T) {
	source := &Item{Category: "Расходники", Subcategory: "---------", SubCategory: "---------"}
	result := itemForPresentation(source)
	if result.Subcategory != "" || result.SubCategory != "" {
		t.Fatalf("placeholder leaked into item presentation: catalog=%q, source=%q", result.Subcategory, result.SubCategory)
	}
	if source.Subcategory != "---------" || source.SubCategory != "---------" {
		t.Fatal("presentation mutated the source item")
	}
	named := itemForPresentation(&Item{Category: "Расходники", Subcategory: "Материалы", SubCategory: "Материалы"})
	if named.Subcategory != "Материалы" || named.SubCategory != "Материалы" {
		t.Fatal("real item subcategory was discarded")
	}
}

func TestPlaceholderSubcategoryNotReturnedByFilters(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	category := ""
	for i := range store.data.Items {
		item := &store.data.Items[i]
		if item.Subcategory == "---------" && item.Category != "Доп. умения" && !isTitleItem(item) && !isTransformationItem(item.ID) {
			if _, isRecipe := store.itemRecipes[item.ID]; !isRecipe {
				category = item.Category
				break
			}
		}
	}
	if category == "" {
		t.Skip("no placeholder records in current catalog")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/items?category="+url.QueryEscape(category), nil)
	recorder := httptest.NewRecorder()
	handleItems(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("items status %d", recorder.Code)
	}
	var payload struct {
		Filters struct {
			Subcategories []string `json:"subcategories"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, value := range payload.Filters.Subcategories {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == "---------" {
			t.Fatalf("placeholder leaked into filter options: %q", value)
		}
	}
}
