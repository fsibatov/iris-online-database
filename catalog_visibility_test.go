package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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

func TestReservedItemsAreHiddenWithoutChangingSource(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"original", "kiss"} {
		for id := 390000351; id <= 390000357; id++ {
			item := store.itemsByID[id]
			if item == nil || item.Name != "reserved" {
				t.Fatalf("reserved source item %d was changed", id)
			}
			for _, query := range []string{"reserved", "Reserved", fmt.Sprint(id)} {
				values := url.Values{"q": {query}, "server": {server}}
				for _, endpoint := range []string{"/api/items", "/api/search"} {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodGet, endpoint+"?"+values.Encode(), nil)
					if endpoint == "/api/items" {
						handleItems(rec, req)
					} else {
						handleSearch(rec, req)
					}
					var result struct {
						Items []Item `json:"items"`
					}
					if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || len(result.Items) != 0 {
						t.Fatalf("reserved item leaked through %s (%s, %q): %s", endpoint, server, query, rec.Body.String())
					}
				}
			}
			rec := httptest.NewRecorder()
			handleItem(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d?server=%s", id, server), nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("reserved detail %d returned %d", id, rec.Code)
			}
			rec = httptest.NewRecorder()
			body := fmt.Sprintf(`{"keys":["item:%d"],"server":%q}`, id, server)
			req := httptest.NewRequest(http.MethodPost, "/api/favorites", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			handleFavorites(rec, req)
			var result struct {
				Rows    []any `json:"rows"`
				Missing int   `json:"missing"`
			}
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || len(result.Rows) != 0 || result.Missing != 1 {
				t.Fatalf("reserved favorite %d was exposed: %s", id, rec.Body.String())
			}
		}
		rec := httptest.NewRecorder()
		values := url.Values{"category": {"Транспорт"}, "server": {server}, "pageSize": {"48"}}
		handleItems(rec, httptest.NewRequest(http.MethodGet, "/api/items?"+values.Encode(), nil))
		var result struct {
			Items []Item `json:"items"`
			Total int    `json:"total"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || result.Total != 20 || len(result.Items) != 20 {
			t.Fatalf("transport count includes reserved entries: %s", rec.Body.String())
		}
		for _, item := range result.Items {
			if strings.EqualFold(item.Name, "reserved") {
				t.Fatal("reserved item appeared in transport catalog")
			}
		}
	}
}

func TestExactCatalogDuplicatesPreserveItemIDsAndDifferentPotions(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.itemDuplicates, map[int]int{300015030: 300015027}) {
		t.Fatalf("unexpected duplicate groups: %v", store.itemDuplicates)
	}
	for _, server := range []string{"original", "kiss"} {
		for _, sample := range []struct {
			query string
			ids   []int
		}{
			{"Бомб", []int{300015027}},
			{"300015030", []int{300015030}},
			{"Благословенное зелье (1 ч)", []int{1100100, 1100500, 91120017}},
		} {
			for _, endpoint := range []string{"/api/items", "/api/search"} {
				values := url.Values{"q": {sample.query}, "server": {server}, "pageSize": {"48"}}
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, endpoint+"?"+values.Encode(), nil)
				if endpoint == "/api/items" {
					handleItems(rec, req)
				} else {
					handleSearch(rec, req)
				}
				var result struct {
					Items []Item `json:"items"`
					Total int    `json:"total"`
				}
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil {
					t.Fatalf("catalog failed: %s", rec.Body.String())
				}
				var ids []int
				for _, item := range result.Items {
					ids = append(ids, item.ID)
				}
				if !reflect.DeepEqual(ids, sample.ids) || (endpoint == "/api/items" && result.Total != len(sample.ids)) {
					t.Fatalf("%s (%s, %q): got %v total=%d, want %v", endpoint, server, sample.query, ids, result.Total, sample.ids)
				}
			}
		}
		for _, id := range []int{300015027, 300015030} {
			rec := httptest.NewRecorder()
			handleItem(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d?server=%s", id, server), nil))
			var result struct {
				Item Item `json:"item"`
			}
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || result.Item.ID != id {
				t.Fatalf("original item ID %d became inaccessible", id)
			}
		}
	}
	for _, value := range []struct{ id, currency, price int }{
		{1100100, 809012, 30}, {1100500, 809013, 4}, {91120017, 809014, 1},
	} {
		item := store.itemsByID[value.id]
		if item.BuyCurrency != value.currency || item.BuyPrice != value.price {
			t.Fatalf("potion variant %d changed", value.id)
		}
	}
}

func TestCatalogDuplicateDetectionProtectsVariantsAndReferences(t *testing.T) {
	for _, sample := range []struct {
		name      string
		change    func(*appStore)
		duplicate bool
	}{
		{"identical", func(*appStore) {}, true},
		{"currency", func(s *appStore) { s.data.Items[0].BuyCurrency++ }, false},
		{"price", func(s *appStore) { s.data.Items[0].BuyPrice++ }, false},
		{"effect", func(s *appStore) { s.data.Items[0].Options = []StatOption{{Type: 213, Value: 9}} }, false},
		{"duration", func(s *appStore) { s.data.Items[0].EffectDurationMs++ }, false},
		{"icon", func(s *appStore) { s.data.Items[0].IconIndex++ }, false},
		{"binding", func(s *appStore) { s.data.Items[0].Exchange++ }, false},
		{"original-drop", func(s *appStore) {
			s.data.Servers = map[string]ServerData{"original": {DropLists: map[string][]DropListEntry{"1": {{ItemID: 102}}}}}
		}, false},
		{"kiss-drop", func(s *appStore) {
			s.data.Servers = map[string]ServerData{"kiss": {DropLists: map[string][]DropListEntry{"1": {{ItemID: 101}}}}}
		}, false},
		{"quest-drop", func(s *appStore) { s.data.QuestDrops = []Drop{{ItemID: 102}} }, false},
		{"quest-reward", func(s *appStore) { s.questRewards = map[int][]questRewardSource{102: {{ItemID: 102}}} }, false},
		{"recipe", func(s *appStore) { s.itemRecipes = map[int][]itemRecipeMaterialSource{102: {{ItemID: 103}}} }, false},
		{"material", func(s *appStore) { s.itemRecipes = map[int][]itemRecipeMaterialSource{103: {{ItemID: 102}}} }, false},
		{"chest", func(s *appStore) {
			s.chestProfiles = map[string]map[int]chestProfileSource{"kiss": {103: {Rows: []chestContentSourceRow{{ItemID: 102}}}}}
		}, false},
		{"set", func(s *appStore) { s.data.ItemSets = map[string]ItemSet{"1": {Items: []ItemSetMember{{ItemID: 102}}}} }, false},
		{"used-currency", func(s *appStore) {
			s.data.Items = append(s.data.Items, Item{ID: 103, Name: "Другой предмет", BuyCurrency: 102})
		}, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			base := Item{ID: 101, Name: "Предмет", BuyCurrency: 809012, BuyPrice: 30, Options: []StatOption{{Type: 213, Value: 8}}}
			copy := base
			copy.ID = 102
			s := appStore{data: GameData{Items: []Item{copy, base}}}
			sample.change(&s)
			before, err := json.Marshal(s.data.Items)
			if err != nil {
				t.Fatal(err)
			}
			s.prepareCatalogDuplicates()
			if sample.duplicate {
				if !reflect.DeepEqual(s.itemDuplicates, map[int]int{102: 101}) {
					t.Fatalf("unstable canonical ID: %v", s.itemDuplicates)
				}
			} else if len(s.itemDuplicates) != 0 {
				t.Fatalf("distinct or referenced items collapsed: %v", s.itemDuplicates)
			}
			after, err := json.Marshal(s.data.Items)
			if err != nil || string(after) != string(before) {
				t.Fatal("duplicate detection changed the source records")
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
