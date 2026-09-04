package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestTitlesCanonicalDataAndLevels(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if len(store.titles) != 312 {
		t.Fatalf("titles=%d want=312", len(store.titles))
	}
	seen := make(map[int]struct{}, len(store.titles))
	withItem := 0
	for index := range store.titles {
		title := &store.titles[index]
		if title.Index <= 0 || strings.TrimSpace(title.Name) == "" {
			t.Fatalf("invalid title: %+v", title)
		}
		if _, exists := seen[title.Index]; exists {
			t.Fatalf("duplicate canonical title index: %d", title.Index)
		}
		seen[title.Index] = struct{}{}
		if title.ItemID > 0 {
			withItem++
			item := store.itemsByID[title.ItemID]
			if item == nil || item.TitleIndex != title.Index {
				t.Fatalf("title %d canonical item mismatch: item=%+v", title.Index, item)
			}
			if titleLevel(title) <= 0 {
				t.Fatalf("title %d has canonical item but no level", title.Index)
			}
		}
	}
	if withItem != 309 {
		t.Fatalf("titles with canonical item=%d want=309", withItem)
	}
	if got := titleLevel(store.titlesByIndex[5]); got != 2 {
		t.Fatalf("title 5 level=%d want=2", got)
	}
	if got := store.titlesByIndex[950].ItemID; got != 1550071 {
		t.Fatalf("title 950 canonical item=%d want=1550071", got)
	}
	for _, index := range []int{300000000, 300000001, 300000010} {
		if title := store.titlesByIndex[index]; title == nil || titleLevel(title) != 0 {
			t.Fatalf("title %d must remain level-unknown without invented data: %+v", index, title)
		}
	}
}

func TestTitlesCatalogSortSearchAndUnknownLevelsLast(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	var all []struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
		Level int    `json:"level"`
	}
	for page := 1; ; page++ {
		req := httptest.NewRequest(http.MethodGet, "/api/titles?sort=level&order=asc&pageSize=48&page="+strconv.Itoa(page), nil)
		rec := httptest.NewRecorder()
		handleTitles(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("page=%d status=%d body=%s", page, rec.Code, rec.Body.String())
		}
		var payload struct {
			Titles []struct {
				Index int    `json:"index"`
				Name  string `json:"name"`
				Level int    `json:"level"`
			} `json:"titles"`
			Pages int `json:"pages"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		all = append(all, payload.Titles...)
		if page >= payload.Pages {
			break
		}
	}
	if len(all) != 312 {
		t.Fatalf("catalog titles=%d want=312", len(all))
	}
	lastLevel := 0
	unknownStarted := false
	for _, title := range all {
		if title.Level == 0 {
			unknownStarted = true
			continue
		}
		if unknownStarted {
			t.Fatalf("known level appears after unknown level: %+v", title)
		}
		if lastLevel > title.Level {
			t.Fatalf("levels not sorted: previous=%d current=%d title=%+v", lastLevel, title.Level, title)
		}
		lastLevel = title.Level
	}

	req := httptest.NewRequest(http.MethodGet, "/api/titles?q="+url.QueryEscape("Одиночка")+"&sort=name&order=asc&pageSize=48", nil)
	rec := httptest.NewRecorder()
	handleTitles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", rec.Code, rec.Body.String())
	}
	var search struct {
		Total  int `json:"total"`
		Titles []struct {
			Index int `json:"index"`
			Level int `json:"level"`
		} `json:"titles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &search); err != nil {
		t.Fatal(err)
	}
	if search.Total != 1 || len(search.Titles) != 1 || search.Titles[0].Index != 5 || search.Titles[0].Level != 2 {
		t.Fatalf("unexpected title search result: %+v", search)
	}
}

func TestTitleItemsAreExcludedFromItemsAndGlobalSearch(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	itemReq := httptest.NewRequest(http.MethodGet, "/api/items?q="+url.QueryEscape("Знак одиночества")+"&pageSize=48", nil)
	itemRec := httptest.NewRecorder()
	handleItems(itemRec, itemReq)
	if itemRec.Code != http.StatusOK {
		t.Fatalf("items status=%d body=%s", itemRec.Code, itemRec.Body.String())
	}
	var itemsPayload struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(itemRec.Body.Bytes(), &itemsPayload); err != nil {
		t.Fatal(err)
	}
	if itemsPayload.Total != 0 {
		t.Fatalf("title item leaked into item catalog: total=%d", itemsPayload.Total)
	}

	filterReq := httptest.NewRequest(http.MethodGet, "/api/items?category="+url.QueryEscape("Расходники")+"&pageSize=48", nil)
	filterRec := httptest.NewRecorder()
	handleItems(filterRec, filterReq)
	if filterRec.Code != http.StatusOK {
		t.Fatalf("filter status=%d body=%s", filterRec.Code, filterRec.Body.String())
	}
	var filterPayload struct {
		Filters struct {
			Subcategories []string `json:"subcategories"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(filterRec.Body.Bytes(), &filterPayload); err != nil {
		t.Fatal(err)
	}
	for _, subcategory := range filterPayload.Filters.Subcategories {
		if subcategory == "Титул" {
			t.Fatal("subcategory Титул leaked into Предметы → Расходники")
		}
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape("Одиночка"), nil)
	searchRec := httptest.NewRecorder()
	handleSearch(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("global search status=%d body=%s", searchRec.Code, searchRec.Body.String())
	}
	var searchPayload struct {
		Items  []map[string]any `json:"items"`
		Titles []struct {
			Index int `json:"index"`
		} `json:"titles"`
	}
	if err := json.Unmarshal(searchRec.Body.Bytes(), &searchPayload); err != nil {
		t.Fatal(err)
	}
	if len(searchPayload.Titles) != 1 || searchPayload.Titles[0].Index != 5 {
		t.Fatalf("title missing from canonical global search: %+v", searchPayload.Titles)
	}
	for _, item := range searchPayload.Items {
		if id, _ := item["id"].(float64); int(id) == 807005 {
			t.Fatal("title sign leaked into global item search")
		}
	}
}

func TestTitleDetailLegacyItemRedirectAndFavoriteMigration(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/titles/5?server=original", nil)
	detailRec := httptest.NewRecorder()
	handleTitle(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("title detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detail struct {
		Title struct {
			Index int    `json:"index"`
			Name  string `json:"name"`
			Level int    `json:"level"`
		} `json:"title"`
		Effect  string `json:"effect"`
		ItemIDs []int  `json:"itemIds"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Title.Index != 5 || detail.Title.Name != "Одиночка" || detail.Title.Level != 2 {
		t.Fatalf("unexpected title detail: %+v", detail.Title)
	}
	if !strings.Contains(detail.Effect, "Физ. уклонение +10") || strings.Contains(detail.Effect, "Получено звание") {
		t.Fatalf("title effect is not clean: %q", detail.Effect)
	}
	if len(detail.ItemIDs) != 1 || detail.ItemIDs[0] != 807005 {
		t.Fatalf("title technical item ids=%v want=[807005]", detail.ItemIDs)
	}

	legacyReq := httptest.NewRequest(http.MethodGet, "/api/items/807005?server=original", nil)
	legacyRec := httptest.NewRecorder()
	handleItem(legacyRec, legacyReq)
	if legacyRec.Code != http.StatusOK {
		t.Fatalf("legacy item status=%d body=%s", legacyRec.Code, legacyRec.Body.String())
	}
	var redirect struct {
		TitleIndex int   `json:"titleIndex"`
		Item       *Item `json:"item"`
	}
	if err := json.Unmarshal(legacyRec.Body.Bytes(), &redirect); err != nil {
		t.Fatal(err)
	}
	if redirect.TitleIndex != 5 || redirect.Item != nil {
		t.Fatalf("legacy title item must redirect without duplicate item detail: %+v", redirect)
	}

	body := []byte(`{"keys":["item:807005","title:5"],"server":"original","page":1,"pageSize":24}`)
	favoriteReq := httptest.NewRequest(http.MethodPost, "/api/favorites", bytes.NewReader(body))
	favoriteReq.Header.Set("Content-Type", "application/json")
	favoriteRec := httptest.NewRecorder()
	handleFavorites(favoriteRec, favoriteReq)
	if favoriteRec.Code != http.StatusOK {
		t.Fatalf("favorites status=%d body=%s", favoriteRec.Code, favoriteRec.Body.String())
	}
	var favorites struct {
		Total int `json:"total"`
		Rows  []struct {
			Kind  string `json:"kind"`
			Index int    `json:"index"`
		} `json:"rows"`
		MigratedKeys map[string]string `json:"migratedKeys"`
	}
	if err := json.Unmarshal(favoriteRec.Body.Bytes(), &favorites); err != nil {
		t.Fatal(err)
	}
	if favorites.Total != 1 || len(favorites.Rows) != 1 || favorites.Rows[0].Kind != "title" || favorites.Rows[0].Index != 5 {
		t.Fatalf("favorites did not deduplicate title: %+v", favorites)
	}
	if favorites.MigratedKeys["item:807005"] != "title:5" {
		t.Fatalf("legacy favorite migration missing: %+v", favorites.MigratedKeys)
	}
}

func TestTitlesKnownSourceFilterAndLevelRange(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	rt := activeRuntime("original")

	var filtered []struct {
		Index int `json:"index"`
		Level int `json:"level"`
	}
	for page := 1; ; page++ {
		req := httptest.NewRequest(http.MethodGet, "/api/titles?server=original&knownSource=1&minLevel=2&maxLevel=30&sort=level&pageSize=48&page="+strconv.Itoa(page), nil)
		rec := httptest.NewRecorder()
		handleTitles(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("page=%d status=%d body=%s", page, rec.Code, rec.Body.String())
		}
		var payload struct {
			Titles []struct {
				Index int `json:"index"`
				Level int `json:"level"`
			} `json:"titles"`
			Pages int `json:"pages"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		filtered = append(filtered, payload.Titles...)
		if page >= payload.Pages {
			break
		}
	}
	if len(filtered) == 0 {
		t.Fatal("known-source title filter unexpectedly returned no titles")
	}
	for _, row := range filtered {
		title := store.titlesByIndex[row.Index]
		if title == nil || !titleHasKnownSource(title, rt) {
			t.Fatalf("title without known source passed filter: %+v", row)
		}
		if row.Level < 2 || row.Level > 30 {
			t.Fatalf("title outside requested level range passed filter: %+v", row)
		}
	}

	badReq := httptest.NewRequest(http.MethodGet, "/api/titles?knownSource=yes", nil)
	badRec := httptest.NewRecorder()
	handleTitles(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid knownSource status=%d want=%d", badRec.Code, http.StatusBadRequest)
	}
}

func TestDuplicateTitleItemsAreAggregatedWithoutDuplicateSources(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	title := store.titlesByIndex[950]
	if title == nil {
		t.Fatal("title 950 missing")
	}
	if len(title.ItemIDs) != 2 || title.ItemIDs[0] != 1550071 || title.ItemIDs[1] != 91121004 {
		t.Fatalf("title 950 item aggregation mismatch: %+v", title.ItemIDs)
	}
	for _, server := range []string{"original", "kiss"} {
		drops := titleDropSources(title, activeRuntime(server))
		seen := make(map[string]struct{}, len(drops))
		for _, drop := range drops {
			key := titleDropKey(drop)
			if _, exists := seen[key]; exists {
				t.Fatalf("duplicate source leaked for server=%s: %+v", server, drop)
			}
			seen[key] = struct{}{}
		}
	}
}

func TestTitleCatalogSortOrderSupportsDescendingAndUnknownLevelsStayLast(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/titles?sort=level&order=desc&pageSize=48&page=1", nil)
	rec := httptest.NewRecorder()
	handleTitles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Titles []struct {
			Index int `json:"index"`
			Level int `json:"level"`
		} `json:"titles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Titles) < 2 {
		t.Fatalf("not enough titles for sort test: %d", len(payload.Titles))
	}
	last := payload.Titles[0].Level
	for _, row := range payload.Titles[1:] {
		if row.Level == 0 {
			continue
		}
		if last == 0 {
			t.Fatalf("known level appeared after unknown level: %+v", row)
		}
		if last < row.Level {
			t.Fatalf("descending levels are broken: prev=%d current=%d", last, row.Level)
		}
		last = row.Level
	}
}
