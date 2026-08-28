package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRecipesCatalogUsesEmbeddedRecipeData(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?page=1&pageSize=24&sort=name", nil)
	rec := httptest.NewRecorder()
	handleRecipes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Recipes []struct {
			ID        int                  `json:"id"`
			Name      string               `json:"name"`
			Materials []ItemRecipeMaterial `json:"materials"`
		} `json:"recipes"`
		Total   int `json:"total"`
		Filters struct {
			Types     []string `json:"types"`
			Qualities []string `json:"qualities"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != len(store.itemRecipes) {
		t.Fatalf("total=%d recipes=%d", payload.Total, len(store.itemRecipes))
	}
	if len(payload.Recipes) == 0 || len(payload.Recipes[0].Materials) == 0 {
		t.Fatalf("recipe rows must include materials: %+v", payload.Recipes)
	}
	if len(payload.Filters.Types) != 4 {
		t.Fatalf("recipe types=%v", payload.Filters.Types)
	}
}

func TestRecipesCatalogSearchesMaterialNames(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	var query string
	for recipeID, materials := range store.itemRecipes {
		recipe := store.itemsByID[recipeID]
		if recipe == nil {
			continue
		}
		for _, material := range materials {
			item := store.itemsByID[material.ItemID]
			if item != nil && item.Name != "" && item.Name != recipe.Name {
				query = item.Name
				break
			}
		}
		if query != "" {
			break
		}
	}
	if query == "" {
		t.Fatal("no searchable material found")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?q="+url.QueryEscape(query)+"&page=1&pageSize=24&sort=name", nil)
	rec := httptest.NewRecorder()
	handleRecipes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total == 0 {
		t.Fatalf("material search %q returned no recipes", query)
	}
}

func TestRecipesKnownSourceFilterAndPreview(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"original", "kiss"} {
		rt := activeRuntime(server)
		expected := 0
		for recipeID := range store.itemRecipes {
			if _, ok := rt.knownSourceItems[recipeID]; ok {
				expected++
			}
		}
		req := httptest.NewRequest(http.MethodGet, "/api/recipes?knownSource=1&server="+server+"&page=1&pageSize=48&sort=name", nil)
		rec := httptest.NewRecorder()
		handleRecipes(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("server=%s status=%d body=%s", server, rec.Code, rec.Body.String())
		}
		var payload struct {
			Total   int `json:"total"`
			Recipes []struct {
				ID            int `json:"id"`
				SourceCount   int `json:"sourceCount"`
				SourcePreview struct {
					Type string `json:"type"`
					Name string `json:"name"`
				} `json:"sourcePreview"`
			} `json:"recipes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Total != expected {
			t.Fatalf("server=%s total=%d expected=%d", server, payload.Total, expected)
		}
		if expected > 0 && len(payload.Recipes) == 0 {
			t.Fatalf("server=%s known-source recipes are empty", server)
		}
		for _, recipe := range payload.Recipes {
			if recipe.SourceCount <= 0 || recipe.SourcePreview.Type == "" || recipe.SourcePreview.Name == "" {
				t.Fatalf("server=%s recipe=%d source preview missing: %+v", server, recipe.ID, recipe)
			}
		}
	}
}

func TestRecipesRejectInvalidKnownSourceFilter(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?knownSource=yes", nil)
	rec := httptest.NewRecorder()
	handleRecipes(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecipesMasterySort(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?page=1&pageSize=48&sort=mastery", nil)
	rec := httptest.NewRecorder()
	handleRecipes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Recipes []struct {
			ID           int `json:"id"`
			Level        int `json:"level"`
			MasteryLevel int `json:"masteryLevel"`
		} `json:"recipes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Recipes) < 2 {
		t.Fatalf("expected multiple recipes, got %d", len(payload.Recipes))
	}
	previous := -1
	for _, recipe := range payload.Recipes {
		item := store.itemsByID[recipe.ID]
		if item == nil {
			t.Fatalf("recipe %d is missing from item store", recipe.ID)
		}
		expected := recipeMasteryLevel(item)
		if recipe.MasteryLevel != expected {
			t.Fatalf("recipe %d mastery=%d makeSkillExp=%d", recipe.ID, recipe.MasteryLevel, item.MakeSkillExp)
		}

		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if recipe.MasteryLevel < previous {
			t.Fatalf("mastery sort is not ascending: %d after %d; recipe=%d name=%q class=%d", recipe.MasteryLevel, previous, recipe.ID, item.Name, catalogNameClass(item.Name))
		}
		previous = recipe.MasteryLevel
	}

	item := store.itemsByID[891219]
	if item == nil || item.Name != "Карта рассеяния I (B)" {
		t.Fatalf("reference recipe missing: %#v", item)
	}
	if item.MakeSkill != 2 || item.MakeSkillExp != 20 || recipeMasteryLevel(item) != 20 {
		t.Fatalf("reference recipe mastery mismatch: makeSkill=%d makeSkillExp=%d mastery=%d", item.MakeSkill, item.MakeSkillExp, recipeMasteryLevel(item))
	}
}

func TestRecipeMasteryUsesProfessionRequirement(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/recipes?q=891219&page=1&pageSize=24&sort=mastery", nil)
	rec := httptest.NewRecorder()
	handleRecipes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Recipes []struct {
			ID           int `json:"id"`
			Level        int `json:"level"`
			MasteryLevel int `json:"masteryLevel"`
			MakeSkill    int `json:"makeSkill"`
		} `json:"recipes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, recipe := range payload.Recipes {
		if recipe.ID != 891219 {
			continue
		}
		found = true
		if recipe.MakeSkill != 2 || recipe.MasteryLevel != 20 {
			t.Fatalf("recipe 891219 expected Каллиграф (20), got makeSkill=%d mastery=%d", recipe.MakeSkill, recipe.MasteryLevel)
		}
		if recipe.Level == recipe.MasteryLevel {
			t.Fatalf("regression fixture must keep item level and mastery distinct, both=%d", recipe.Level)
		}
	}
	if !found {
		t.Fatal("recipe 891219 not returned by recipes API")
	}
}

func TestRecipeProductResolutionUsesOnlyUniqueFinishedItem(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		recipeID  int
		productID int
	}{
		{recipeID: 891001, productID: 880001},
		{recipeID: 891219, productID: 1050007},
		{recipeID: 891401, productID: 835206},
	}
	for _, tc := range cases {
		recipe := store.itemsByID[tc.recipeID]
		if recipe == nil {
			t.Fatalf("recipe %d missing", tc.recipeID)
		}
		product := recipeProduct(recipe)
		if product == nil || product.ID != tc.productID {
			t.Fatalf("recipe %d product=%#v want=%d", tc.recipeID, product, tc.productID)
		}
	}

	ambiguous := store.itemsByID[891415]
	if ambiguous == nil {
		t.Fatal("ambiguous recipe fixture missing")
	}
	if product := recipeProduct(ambiguous); product != nil {
		t.Fatalf("ambiguous recipe resolved unexpectedly to %d", product.ID)
	}
}

func TestRecipeItemDetailIncludesFinishedItemEffectData(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items/891001?server=original", nil)
	rec := httptest.NewRecorder()
	handleItem(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		RecipeProduct *struct {
			Item Item `json:"item"`
		} `json:"recipeProduct"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RecipeProduct == nil {
		t.Fatal("recipeProduct missing")
	}
	if payload.RecipeProduct.Item.ID != 880001 {
		t.Fatalf("product id=%d", payload.RecipeProduct.Item.ID)
	}
	if !strings.Contains(payload.RecipeProduct.Item.AbilityDescription, "+10") || !strings.Contains(strings.ToLower(payload.RecipeProduct.Item.AbilityDescription), "регенерац") {
		t.Fatalf("finished item effect missing: %q", payload.RecipeProduct.Item.AbilityDescription)
	}
}

func TestItemsFilterHidesAdditionalSkillsCategory(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items?page=1&pageSize=24&sort=name", nil)
	rec := httptest.NewRecorder()
	handleItems(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Filters struct {
			Categories []string `json:"categories"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, category := range payload.Filters.Categories {
		if category == "Доп. умения" {
			t.Fatalf("Доп. умения must be hidden from item filters: %v", payload.Filters.Categories)
		}
	}
}

func TestItemsCatalogAndGlobalSearchExcludeRecipes(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	itemsReq := httptest.NewRequest(http.MethodGet, "/api/items?page=1&pageSize=48&sort=name", nil)
	itemsRec := httptest.NewRecorder()
	handleItems(itemsRec, itemsReq)
	if itemsRec.Code != http.StatusOK {
		t.Fatalf("items status=%d body=%s", itemsRec.Code, itemsRec.Body.String())
	}
	var itemsPayload struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(itemsRec.Body.Bytes(), &itemsPayload); err != nil {
		t.Fatal(err)
	}
	expected := 0
	for index := range store.data.Items {
		item := &store.data.Items[index]
		if _, isRecipe := store.itemRecipes[item.ID]; isRecipe || isTitleItem(item) || isTransformationItem(item.ID) {
			continue
		}
		expected++
	}
	if itemsPayload.Total != expected {
		t.Fatalf("items total=%d expected=%d", itemsPayload.Total, expected)
	}

	var recipe *Item
	for recipeID := range store.itemRecipes {
		candidate := store.itemsByID[recipeID]
		if candidate != nil && strings.TrimSpace(candidate.Name) != "" {
			recipe = candidate
			break
		}
	}
	if recipe == nil {
		t.Fatal("no recipe item found")
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(recipe.Name)+"&server=original", nil)
	searchRec := httptest.NewRecorder()
	handleSearch(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", searchRec.Code, searchRec.Body.String())
	}
	var searchPayload struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(searchRec.Body.Bytes(), &searchPayload); err != nil {
		t.Fatal(err)
	}
	for _, item := range searchPayload.Items {
		if item.ID == recipe.ID {
			t.Fatalf("recipe %d leaked into item search results", recipe.ID)
		}
	}
}

func TestFavoriteRecipeKeepsRecipePresentation(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	var recipeID int
	for id := range store.itemRecipes {
		if store.itemsByID[id] != nil {
			recipeID = id
			break
		}
	}
	if recipeID == 0 {
		t.Fatal("no recipe item found")
	}

	body, err := json.Marshal(map[string]any{
		"keys":     []string{fmt.Sprintf("item:%d", recipeID)},
		"server":   "original",
		"page":     1,
		"pageSize": 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/favorites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleFavorites(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Rows []struct {
			Kind      string `json:"kind"`
			ID        int    `json:"id"`
			Materials []any  `json:"materials"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 {
		t.Fatalf("rows=%d want=1", len(payload.Rows))
	}
	if payload.Rows[0].Kind != "recipe" || payload.Rows[0].ID != recipeID {
		t.Fatalf("favorite recipe presentation=%#v", payload.Rows[0])
	}
	if payload.Rows[0].Materials == nil {
		t.Fatal("recipe favorite lost recipe materials")
	}
}
