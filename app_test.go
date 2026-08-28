package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCatalogNameSortingPutsRussianThenOtherLanguagesThenDigitsSymbolsBlank(t *testing.T) {
	names := []string{
		"",
		"[поддержка поселенцев] Рыцарский меч",
		"210100",
		"Zulu",
		"Ядовитый паук",
		"Ёж",
		"Ель",
		"Жук",
		"Alpha",
		"Альфа",
		"*призыв разлома*",
		"   ",
	}
	sort.SliceStable(names, func(i, j int) bool {
		return catalogNameLess(names[i], names[j])
	})
	want := []string{
		"Альфа",
		"Ель",
		"Ёж",
		"Жук",
		"Ядовитый паук",
		"Alpha",
		"Zulu",
		"210100",
		"*призыв разлома*",
		"[поддержка поселенцев] Рыцарский меч",
		"",
		"   ",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("catalog name order = %#v, want %#v", names, want)
	}
}

func TestItemFilterCategoryAndQualityOrdering(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	categoryReq := httptest.NewRequest(http.MethodGet, "/api/items?pageSize=8", nil)
	categoryRecorder := httptest.NewRecorder()
	handleItems(categoryRecorder, categoryReq)
	if categoryRecorder.Code != http.StatusOK {
		t.Fatalf("category status=%d: %s", categoryRecorder.Code, categoryRecorder.Body.String())
	}
	var categoryResponse struct {
		Filters struct {
			Categories []string `json:"categories"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(categoryRecorder.Body.Bytes(), &categoryResponse); err != nil {
		t.Fatal(err)
	}
	wantCategories := []string{"Оружие/щит", "Броня (кожа)", "Броня (латы)", "Броня (ткань)", "Бижутерия"}
	if len(categoryResponse.Filters.Categories) < len(wantCategories) || !reflect.DeepEqual(categoryResponse.Filters.Categories[:len(wantCategories)], wantCategories) {
		t.Fatalf("leading item categories = %#v, want prefix %#v", categoryResponse.Filters.Categories, wantCategories)
	}

	qualityReq := httptest.NewRequest(http.MethodGet, "/api/items?category="+url.QueryEscape("Бижутерия")+"&pageSize=8", nil)
	qualityRecorder := httptest.NewRecorder()
	handleItems(qualityRecorder, qualityReq)
	if qualityRecorder.Code != http.StatusOK {
		t.Fatalf("quality status=%d: %s", qualityRecorder.Code, qualityRecorder.Body.String())
	}
	var qualityResponse struct {
		Filters struct {
			Qualities []string `json:"qualities"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(qualityRecorder.Body.Bytes(), &qualityResponse); err != nil {
		t.Fatal(err)
	}
	wantQualities := []string{"Не указано", "Обычное", "Необычное", "Редкое", "Уникальное", "PvP", "Эпическое", "Особое", "Событийное"}
	if !reflect.DeepEqual(qualityResponse.Filters.Qualities, wantQualities) {
		t.Fatalf("quality filter order = %#v, want %#v", qualityResponse.Filters.Qualities, wantQualities)
	}
}

func TestItemRaritySortRunsFromLowestQualityIDToHighest(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items?sort=rarity&page=1&pageSize=48", nil)
	recorder := httptest.NewRecorder()
	handleItems(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) == 0 {
		t.Fatal("rarity-sorted catalog is empty")
	}
	for i := 1; i < len(response.Items); i++ {
		if response.Items[i-1].QualityID > response.Items[i].QualityID {
			t.Fatalf("rarity order decreased at %d: %d > %d", i, response.Items[i-1].QualityID, response.Items[i].QualityID)
		}
	}
}

func TestMonsterNameCatalogPlacesBlankNamesLast(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/monsters?server=kiss&sort=name&page=1&pageSize=48", nil)
	recorder := httptest.NewRecorder()
	handleMonsters(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var first struct {
		Monsters []Monster `json:"monsters"`
		Total    int       `json:"total"`
		Pages    int       `json:"pages"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	for _, monster := range first.Monsters {
		if strings.TrimSpace(monster.Name) == "" {
			t.Fatalf("blank-name monster %d appeared on first alphabetical page", monster.ID)
		}
	}

	lastReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/monsters?server=kiss&sort=name&page=%d&pageSize=48", first.Pages), nil)
	lastRecorder := httptest.NewRecorder()
	handleMonsters(lastRecorder, lastReq)
	if lastRecorder.Code != http.StatusOK {
		t.Fatalf("last page status=%d: %s", lastRecorder.Code, lastRecorder.Body.String())
	}
	var last struct {
		Monsters []Monster `json:"monsters"`
	}
	if err := json.Unmarshal(lastRecorder.Body.Bytes(), &last); err != nil {
		t.Fatal(err)
	}
	blankFound := false
	for _, monster := range last.Monsters {
		if strings.TrimSpace(monster.Name) == "" {
			blankFound = true
		}
	}
	if !blankFound {
		t.Fatal("expected at least one blank-name monster on the final alphabetical page")
	}
}

func TestWeaponPresentationUsesSourceWeaponSubtype(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	type expectedPresentation struct {
		subcategory string
		typeLine    string
	}
	expectedFor := func(item *Item) (expectedPresentation, bool) {
		if item.Category != "Оружие/щит" || item.MainCategoryID != 1 || item.MiddleCategoryID == 602 {
			return expectedPresentation{}, false
		}
		switch item.EventType {
		case 0:
			return expectedPresentation{"Мечи", "[Оружие] Меч"}, true
		case 1:
			return expectedPresentation{"Двуручные мечи", "[Оружие] Двуручный меч"}, true
		case 2:
			return expectedPresentation{"Сабли", "[Оружие] Сабля"}, true
		case 3:
			return expectedPresentation{"Кинжалы", "[Оружие] Кинжал"}, true
		case 4:
			return expectedPresentation{"Ружья", "[Оружие] Ружьё"}, true
		case 5:
			if item.MiddleCategoryID == 107 {
				return expectedPresentation{"Скипетры", "[Оружие] Скипетр"}, true
			}
			return expectedPresentation{"Посохи", "[Оружие] Посох"}, true
		case 6:
			return expectedPresentation{"Щиты", "[Оружие] Щит"}, true
		default:
			return expectedPresentation{}, false
		}
	}

	counts := map[string]int{}
	checked := 0
	for index := range store.data.Items {
		item := &store.data.Items[index]
		expected, ok := expectedFor(item)
		if !ok {
			continue
		}
		checked++
		presented := itemForPresentation(item)
		if presented.Subcategory != expected.subcategory || presented.TypeLine != expected.typeLine {
			t.Fatalf("item %d (%q), source subtype %d: presentation = %q / %q, want %q / %q", item.ID, item.Name, item.EventType, presented.Subcategory, presented.TypeLine, expected.subcategory, expected.typeLine)
		}
		counts[presented.Subcategory]++
	}
	if checked != 1830 {
		t.Fatalf("verified weapon count = %d, want 1830", checked)
	}
	wantCounts := map[string]int{
		"Мечи":           233,
		"Двуручные мечи": 229,
		"Сабли":          229,
		"Кинжалы":        230,
		"Ружья":          230,
		"Посохи":         230,
		"Скипетры":       228,
		"Щиты":           221,
	}
	if !reflect.DeepEqual(counts, wantCounts) {
		t.Fatalf("weapon presentation counts = %#v, want %#v", counts, wantCounts)
	}

	for subcategory, wantTotal := range wantCounts {
		query := url.Values{
			"category":    {"Оружие/щит"},
			"subcategory": {subcategory},
			"pageSize":    {"48"},
		}
		req := httptest.NewRequest(http.MethodGet, "/api/items?"+query.Encode(), nil)
		recorder := httptest.NewRecorder()
		handleItems(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s filter status=%d: %s", subcategory, recorder.Code, recorder.Body.String())
		}
		var payload struct {
			Total   int `json:"total"`
			Filters struct {
				Subcategories []string `json:"subcategories"`
			} `json:"filters"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Total != wantTotal {
			t.Fatalf("%s filter total = %d, want %d", subcategory, payload.Total, wantTotal)
		}
		if slices.Contains(payload.Filters.Subcategories, "Парные клинки") {
			t.Fatal("obsolete Парные клинки classification is still exposed")
		}
	}

	cases := map[int]expectedPresentation{
		1201:      {"Мечи", "[Оружие] Меч"},
		10201:     {"Двуручные мечи", "[Оружие] Двуручный меч"},
		20001:     {"Сабли", "[Оружие] Сабля"},
		21252:     {"Сабли", "[Оружие] Сабля"},
		31652:     {"Кинжалы", "[Оружие] Кинжал"},
		42407:     {"Ружья", "[Оружие] Ружьё"},
		145502001: {"Посохи", "[Оружие] Посох"},
		145602001: {"Скипетры", "[Оружие] Скипетр"},
		80008:     {"Щиты", "[Оружие] Щит"},
		81411:     {"Сабли", "[Оружие] Сабля"},
	}
	for id, expected := range cases {
		item := store.itemsByID[id]
		if item == nil {
			t.Fatalf("weapon %d is missing", id)
		}
		presented := itemForPresentation(item)
		if presented.Subcategory != expected.subcategory || presented.TypeLine != expected.typeLine {
			t.Fatalf("weapon %d presentation = %q / %q, want %q / %q", id, presented.Subcategory, presented.TypeLine, expected.subcategory, expected.typeLine)
		}
	}

	tool := store.itemsByID[878004]
	if tool == nil {
		t.Fatal("mining tool 878004 is missing")
	}
	presentedTool := itemForPresentation(tool)
	if presentedTool.Subcategory != "Инструменты" || presentedTool.TypeLine != "[Оружие] Инструмент" {
		t.Fatalf("mining tool presentation changed unexpectedly: %q / %q", presentedTool.Subcategory, presentedTool.TypeLine)
	}
}

func TestKaratSabersPresentationIsConsistentAcrossEndpoints(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	item := store.itemsByID[81411]
	if item == nil {
		t.Fatal("karat sabers 81411 are missing")
	}
	if item.MiddleCategoryID != 103 || item.MiddleCategory != "Два клинка" || item.EventType != 2 || item.Category != "Оружие/щит" {
		t.Fatalf("karat sabers raw source projection changed unexpectedly: middle=%d %q eventType=%d category=%q", item.MiddleCategoryID, item.MiddleCategory, item.EventType, item.Category)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/items/81411?server=original", nil)
	detailRecorder := httptest.NewRecorder()
	handleItem(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status=%d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail struct {
		Item Item `json:"item"`
	}
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Item.Subcategory != "Сабли" || detail.Item.TypeLine != "[Оружие] Сабля" {
		t.Fatalf("detail presentation = %q / %q", detail.Item.Subcategory, detail.Item.TypeLine)
	}

	recipeRequest := httptest.NewRequest(http.MethodGet, "/api/items/3000102?server=original", nil)
	recipeRecorder := httptest.NewRecorder()
	handleItem(recipeRecorder, recipeRequest)
	if recipeRecorder.Code != http.StatusOK {
		t.Fatalf("recipe detail status=%d: %s", recipeRecorder.Code, recipeRecorder.Body.String())
	}
	var recipeDetail struct {
		RecipeProduct *struct {
			Item Item `json:"item"`
		} `json:"recipeProduct"`
	}
	if err := json.Unmarshal(recipeRecorder.Body.Bytes(), &recipeDetail); err != nil {
		t.Fatal(err)
	}
	if recipeDetail.RecipeProduct == nil || recipeDetail.RecipeProduct.Item.ID != 81411 {
		t.Fatal("karat saber recipe product was not resolved")
	}
	if recipeDetail.RecipeProduct.Item.Subcategory != "Сабли" || recipeDetail.RecipeProduct.Item.TypeLine != "[Оружие] Сабля" {
		t.Fatalf("recipe product presentation = %q / %q", recipeDetail.RecipeProduct.Item.Subcategory, recipeDetail.RecipeProduct.Item.TypeLine)
	}

	query := url.Values{
		"category":    {"Оружие/щит"},
		"subcategory": {"Сабли"},
		"q":           {"Собранный драгоценные мечи карата"},
		"pageSize":    {"48"},
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/items?"+query.Encode(), nil)
	listRecorder := httptest.NewRecorder()
	handleItems(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("saber filter status=%d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var list struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range list.Items {
		if candidate.ID == 81411 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("karat sabers are missing from the Сабли filter")
	}

	query.Set("subcategory", "Мечи")
	wrongListRequest := httptest.NewRequest(http.MethodGet, "/api/items?"+query.Encode(), nil)
	wrongListRecorder := httptest.NewRecorder()
	handleItems(wrongListRecorder, wrongListRequest)
	if wrongListRecorder.Code != http.StatusOK {
		t.Fatalf("sword filter status=%d: %s", wrongListRecorder.Code, wrongListRecorder.Body.String())
	}
	list.Items = nil
	if err := json.Unmarshal(wrongListRecorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range list.Items {
		if candidate.ID == 81411 {
			t.Fatal("karat sabers leak into the Мечи filter")
		}
	}
}

func TestDependentItemFiltersAreIgnoredWithoutCategory(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/items?subcategory=Мечи&quality=Редкий&pageSize=8", nil)
	recorder := httptest.NewRecorder()
	handleItems(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []Item `json:"items"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
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
	if response.Total != expected {
		t.Fatalf("dependent filters were applied without a category: got %d want %d", response.Total, expected)
	}
}

func TestKnownSourceItemFilterIsServerAware(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	kiss := activeRuntime("kiss")
	original := activeRuntime("original")
	if len(kiss.knownSourceItems) == 0 || len(original.knownSourceItems) == 0 {
		t.Fatalf("known-source index is empty: kiss=%d original=%d", len(kiss.knownSourceItems), len(original.knownSourceItems))
	}

	eligibleCatalogItem := func(id int) bool {
		item := store.itemsByID[id]
		if item == nil || isTitleItem(item) {
			return false
		}
		_, isRecipe := store.itemRecipes[id]
		return !isRecipe
	}
	type sourceCandidate struct {
		id           int
		sourceServer string
		otherServer  string
	}
	candidates := make([]sourceCandidate, 0)
	for id := range kiss.knownSourceItems {
		if _, ok := original.knownSourceItems[id]; ok || !eligibleCatalogItem(id) {
			continue
		}
		candidates = append(candidates, sourceCandidate{id: id, sourceServer: "kiss", otherServer: "original"})
	}
	for id := range original.knownSourceItems {
		if _, ok := kiss.knownSourceItems[id]; ok || !eligibleCatalogItem(id) {
			continue
		}
		candidates = append(candidates, sourceCandidate{id: id, sourceServer: "original", otherServer: "kiss"})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].id != candidates[j].id {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].sourceServer < candidates[j].sourceServer
	})
	if len(candidates) == 0 {
		t.Fatal("no catalog item with a server-specific known source found in the current data package")
	}
	itemID := candidates[0].id
	sourceServer := candidates[0].sourceServer
	otherServer := candidates[0].otherServer

	request := func(server string) struct {
		Items []Item `json:"items"`
		Total int    `json:"total"`
	} {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items?q=%d&knownSource=1&server=%s&pageSize=48", itemID, server), nil)
		recorder := httptest.NewRecorder()
		handleItems(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("server=%s status=%d: %s", server, recorder.Code, recorder.Body.String())
		}
		var response struct {
			Items []Item `json:"items"`
			Total int    `json:"total"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	contains := func(items []Item, id int) bool {
		for _, item := range items {
			if item.ID == id {
				return true
			}
		}
		return false
	}
	if response := request(sourceServer); !contains(response.Items, itemID) {
		t.Fatalf("item %d with a known source on %s was filtered out: %#v", itemID, sourceServer, response.Items)
	}
	if response := request(otherServer); contains(response.Items, itemID) {
		t.Fatalf("item %d incorrectly has a known source on %s", itemID, otherServer)
	}
}

func TestKnownSourceItemFilterRejectsInvalidValue(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items?knownSource=yes", nil)
	recorder := httptest.NewRecorder()
	handleItems(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", recorder.Code, http.StatusBadRequest)
	}
}

func TestDependentMonsterTypeIgnoredWithoutCategory(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/monsters?type=Нежить&pageSize=8", nil)
	recorder := httptest.NewRecorder()
	handleMonsters(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	const want = 677
	if response.Total != want {
		t.Fatalf("dependent type was applied without a category or server visibility was lost: got %d want %d", response.Total, want)
	}
}

func TestMonster10042PreservesSevenDirectSlots(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	runtime := activeRuntime("kiss")
	_, slots := monsterDropView(store.monstersByID[10042], runtime)
	direct := make([]DropSlot, 0, 7)
	for _, slot := range slots {
		if slot.Source == "Выпадение монстра" {
			direct = append(direct, slot)
		}
	}
	if len(direct) != 7 {
		t.Fatalf("direct slots=%d want=7", len(direct))
	}
	first := direct[0]
	if len(first.Choices) != 3 {
		t.Fatalf("first slot choices=%d want=3", len(first.Choices))
	}
	wantGroups := []int{1004002, 1004003, 1004004}
	wantChances := []float64{40, 35, 25}
	for index := range wantGroups {
		if first.Choices[index].GroupID != wantGroups[index] || first.Choices[index].Chance != wantChances[index] {
			t.Fatalf("first slot choice %d=%#v", index, first.Choices[index])
		}
	}
	last := direct[6]
	if last.ChanceOverflow || last.ChanceTotal != 100 || len(last.Choices) != 1 || last.Choices[0].GroupID != 6000501 {
		t.Fatalf("last slot was not reduced to one guaranteed group: %#v", last)
	}
}

func TestDropListOrderAndQuantityArePreserved(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	runtime := activeRuntime("kiss")
	_, slots := monsterDropView(store.monstersByID[10042], runtime)
	for _, slot := range slots {
		for _, choice := range slot.Choices {
			if choice.GroupID != 2300304 {
				continue
			}
			if len(choice.Items) != 3 {
				t.Fatalf("group items=%d want=3", len(choice.Items))
			}
			wantQuantities := []int{1, 3, 5}
			wantChances := []float64{85, 10, 5}
			for index, item := range choice.Items {
				if item.Position != index+1 || item.Quantity != wantQuantities[index] || item.Chance != wantChances[index] {
					t.Fatalf("item %d=%#v", index, item)
				}
			}
			return
		}
	}
	t.Fatal("group 2300304 was not found")
}

func TestMonsterEndpointIncludesSlotModel(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/monsters/10042?server=kiss", nil)
	recorder := httptest.NewRecorder()
	handleMonster(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Slots []DropSlot `json:"slots"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	direct := 0
	for _, slot := range response.Slots {
		if slot.Source == "Выпадение монстра" {
			direct++
		}
	}
	if direct != 7 {
		t.Fatalf("endpoint direct slots=%d want=7", direct)
	}
}

func TestOrderedDropChanceUsesServerCumulativeBoundaries(t *testing.T) {
	weights := []float64{20, 85, 10}
	if got := orderedEffectiveChance(weights, 0); got != 20 {
		t.Fatalf("first effective chance=%v want=20", got)
	}
	if got := orderedEffectiveChance(weights, 1); got != 80 {
		t.Fatalf("second effective chance=%v want=80", got)
	}
	if got := orderedEffectiveChance(weights, 2); got != 0 {
		t.Fatalf("overflow tail effective chance=%v want=0", got)
	}
	encoded, err := json.Marshal(ItemDrop{GroupChance: 85, GroupBaseChance: 80, ItemChance: 50, ItemBaseChance: 50, BaseAttemptChance: 40})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"effectiveChance"`) || !strings.Contains(string(encoded), `"baseAttemptChance":40`) {
		t.Fatalf("drop API exposes misleading or missing chance fields: %s", encoded)
	}
}

func TestSpikyOwlBattleLeggingsBaseAttemptChance(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	var monster *Monster
	for index := range store.data.Monsters {
		if store.data.Monsters[index].ID == 108 {
			monster = &store.data.Monsters[index]
			break
		}
	}
	if monster == nil || monster.Name != "Шипастая сова" {
		t.Fatalf("monster 108 mismatch: %#v", monster)
	}
	_, slots := monsterDropView(monster, activeRuntime("original"))
	for _, slot := range slots {
		for _, choice := range slot.Choices {
			for _, item := range choice.Items {
				if item.ItemID != 241651 {
					continue
				}
				if math.Abs(choice.BaseSelectionChance-0.0042) > 1e-12 {
					t.Fatalf("group base chance=%0.12f want 0.0042", choice.BaseSelectionChance)
				}
				if math.Abs(item.BaseSelectionChance-0.0833) > 1e-12 {
					t.Fatalf("item base chance=%0.12f want 0.0833", item.BaseSelectionChance)
				}
				if math.Abs(item.BaseAttemptChance-0.0000034986) > 1e-15 {
					t.Fatalf("base attempt chance=%0.12f want 0.0000034986", item.BaseAttemptChance)
				}
				return
			}
		}
	}
	t.Fatal("Поножи со следами битв not found in Шипастая сова drop model")
}

func TestItemDropSourcesUseRequestedSourceAndMonsterOrder(t *testing.T) {
	drops := []ItemDrop{
		{Monster: "Бета", MonsterID: 20, MonsterLevel: 30, Source: "Выпадение монстра", GroupChanceKnown: true, BaseAttemptChance: 25},
		{Monster: "Гамма", MonsterID: 30, MonsterLevel: 10, Source: "Выпадение монстра", GroupChanceKnown: true, BaseAttemptChance: 75},
		{Monster: "Альфа", MonsterID: 10, MonsterLevel: 20, Source: "Выпадение монстра", GroupChanceKnown: true, BaseAttemptChance: 25},
		{Monster: "Аарон", MonsterID: 5, MonsterLevel: 20, Source: "Выпадение монстра", GroupChanceKnown: true, BaseAttemptChance: 25},
		{Monster: "Мировой", Source: "Мировое выпадение", GroupChanceKnown: true, BaseAttemptChance: 99},
		{Container: "Сундук", ContainerID: 808094, Source: "Сундук", ItemBaseChance: 100},
		{Quest: "Квест", Source: "Квестовое выпадение", ItemBaseChance: 100},
	}

	sortItemDropSources(drops)
	want := []struct {
		source string
		id     int
	}{
		{"Выпадение монстра", 30},
		{"Выпадение монстра", 5},
		{"Выпадение монстра", 10},
		{"Выпадение монстра", 20},
		{"Мировое выпадение", 0},
		{"Сундук", 0},
		{"Квестовое выпадение", 0},
	}
	for index := range want {
		if drops[index].Source != want[index].source || drops[index].MonsterID != want[index].id {
			t.Fatalf("source %d=%q monster=%d want=%q/%d", index, drops[index].Source, drops[index].MonsterID, want[index].source, want[index].id)
		}
	}
}

func TestItemSourceInterfacePreservesBackendOrder(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, marker := range []string{
		"const drops = [...(data.drops || [])]",
		"buildSourceSections(drops)",
		"section.rows.slice(0, section.shown)",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("source order marker is missing: %s", marker)
		}
	}
	if strings.Contains(script, "sort((a, b) => baseAttemptChance(b) - baseAttemptChance(a))") {
		t.Fatal("UI re-sorts sources globally and can break source-type ordering")
	}
}

func TestItemInterfaceShowsAllSourcesGroupedByType(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	sectionStart := strings.Index(script, "function buildSourceSections")
	sectionEnd := strings.Index(script[sectionStart:], "function chestSourceDetails")
	if sectionStart < 0 || sectionEnd < 0 {
		t.Fatal("buildSourceSections function is missing")
	}
	sectionCode := script[sectionStart : sectionStart+sectionEnd]
	positions := make([]int, 0, 4)
	for _, expected := range []string{"Монстры с подтверждённым выпадением", "Мировая добыча", "Сундуки", "Задания"} {
		position := strings.Index(sectionCode, expected)
		if position < 0 {
			t.Fatalf("source group marker is missing: %s", expected)
		}
		positions = append(positions, position)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Fatalf("source groups are not in requested order: %#v", positions)
		}
	}
	for _, expected := range []string{"Квестовый дроп", "SOURCE_BATCH", "data-source-more", "world-source-compact"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("source UI marker is missing: %s", expected)
		}
	}
	if !strings.Contains(script, "drops || []") {
		t.Fatal("item detail does not preserve the complete source response")
	}
}

func TestItemEndpointReturnsEveryComputedSource(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	const itemID = 809012
	const want = 277
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%d?server=kiss", itemID), nil)
	recorder := httptest.NewRecorder()
	handleItem(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Drops []ItemDrop `json:"drops"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Drops) != want {
		t.Fatalf("endpoint sources=%d want=%d", len(response.Drops), want)
	}
}

func TestPlayerFacingDropTerminologyExplainsIndependentAttempts(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	interfaceSource, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	combined := strings.ToLower(string(mainSource) + string(interfaceSource))
	for _, expected := range []string{"отдельная попытка выпадения", "Шанс группы", "оба выбора сработают подряд"} {
		if !strings.Contains(combined, strings.ToLower(expected)) {
			t.Fatalf("drop explanation marker is missing: %s", expected)
		}
	}
	if strings.Contains(combined, "ячейка выпадения") || strings.Contains(combined, "ячейки выпадения") {
		t.Fatal("player-facing cell terminology remains")
	}
}

func TestEmbeddedDataDoesNotContainFlattenedDirectRules(t *testing.T) {
	file, err := os.Open("assets/game_data.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte(`"directRules"`)) {
		t.Fatal("embedded data still contains flattened directRules")
	}
}

func TestPublicInterfaceDoesNotExposeServerTableNames(t *testing.T) {
	paths := []string{"main.go", "web/app.js", "web/index.html"}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"item_dropn.txt", "item_droplist.txt", "item_dropw.txt"} {
			if strings.Contains(string(data), forbidden) {
				t.Fatalf("%s exposes confidential table name %s", path, forbidden)
			}
		}
	}
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"item_dropn.txt", "item_droplist.txt", "item_dropw.txt"} {
		if strings.Contains(store.data.Meta.DropNote, forbidden) {
			t.Fatalf("embedded metadata exposes confidential table name %s", forbidden)
		}
	}
}

func TestProfileAtomicWriteAndBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	backup := filepath.Join(dir, "Backups", "profile.json.bak")
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		t.Fatal(err)
	}
	first := defaultProfile()
	first.Favorites = []string{"item:1"}
	if err := atomicWriteJSON(path, backup, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Favorites = []string{"monster:2"}
	if err := atomicWriteJSON(path, backup, second); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Favorites) != 1 || loaded.Favorites[0] != "monster:2" {
		t.Fatalf("unexpected current profile: %#v", loaded.Favorites)
	}
	previous, err := loadProfileFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if len(previous.Favorites) != 1 || previous.Favorites[0] != "item:1" {
		t.Fatalf("unexpected backup profile: %#v", previous.Favorites)
	}
}

func TestResponseCacheBoundedAndExpires(t *testing.T) {
	cache := newResponseCache(2, 16, 20*time.Millisecond)
	cache.Put("a", 200, nil, []byte("1234"))
	cache.Put("b", 200, nil, []byte("5678"))
	cache.Put("c", 200, nil, []byte("9012"))
	if _, ok := cache.Get("a"); ok {
		t.Fatal("least recently used entry was not evicted")
	}
	time.Sleep(25 * time.Millisecond)
	if _, ok := cache.Get("c"); ok {
		t.Fatal("expired cache entry was returned")
	}
}

func TestCorruptProfileFallsBackToBackup(t *testing.T) {
	dir := t.TempDir()
	paths := appPaths{Profile: filepath.Join(dir, "UserData", "profile.json"), Backups: filepath.Join(dir, "UserData", "Backups")}
	if err := os.MkdirAll(paths.Backups, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := defaultProfile()
	backup.Favorites = []string{"item:77"}
	if err := atomicWriteJSON(filepath.Join(paths.Backups, "profile.json.bak"), filepath.Join(paths.Backups, "unused.bak"), backup); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.Profile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Profile, []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(paths)
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Get()
	if len(profile.Favorites) != 1 || profile.Favorites[0] != "item:77" {
		t.Fatalf("backup not restored: %#v", profile.Favorites)
	}
}

func TestEmptyProfileFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	paths := appPaths{Profile: filepath.Join(dir, "profile.json"), Backups: filepath.Join(dir, "Backups")}
	if err := os.WriteFile(paths.Profile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(paths)
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Get()
	if profile.SchemaVersion != profileSchemaVersion || profile.Settings.Server != "kiss" || profile.Settings.Theme != "dark" || profile.Settings.View != "list" || len(profile.Favorites) != 0 || len(profile.History) != 0 {
		t.Fatalf("empty profile did not fall back to defaults: %#v", profile)
	}
}

func TestTruncatedProfileFallsBackToBackup(t *testing.T) {
	dir := t.TempDir()
	paths := appPaths{Profile: filepath.Join(dir, "UserData", "profile.json"), Backups: filepath.Join(dir, "UserData", "Backups")}
	if err := os.MkdirAll(paths.Backups, 0o700); err != nil {
		t.Fatal(err)
	}
	backup := defaultProfile()
	backup.Settings.Server = "original"
	backup.Settings.Theme = "light"
	backup.Favorites = []string{"item:80567"}
	if err := atomicWriteJSON(filepath.Join(paths.Backups, "profile.json.bak"), filepath.Join(paths.Backups, "unused.bak"), backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Profile, []byte(`{"schemaVersion":1,"settings":`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(paths)
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Get()
	if profile.Settings.Server != "original" || profile.Settings.Theme != "light" || len(profile.Favorites) != 1 {
		t.Fatalf("truncated profile did not restore backup: %#v", profile)
	}
}

func TestProfileWriteFailureKeepsInMemoryState(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: filepath.Join(blocker, "profile.json"), Backups: filepath.Join(blocker, "Backups")})
	if err != nil {
		t.Fatal(err)
	}
	before := store.Get()
	updated := before
	updated.Settings.Server = "original"
	updated.Favorites = []string{"item:80567"}
	if err := store.Replace(updated); err == nil {
		t.Fatal("profile write unexpectedly succeeded through a non-directory parent")
	}
	after := store.Get()
	if after.Settings.Server != before.Settings.Server || len(after.Favorites) != len(before.Favorites) {
		t.Fatalf("failed write changed in-memory profile: before=%#v after=%#v", before, after)
	}
}

func TestCleanupDoesNotFollowSymlink(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Cache")
	outside := filepath.Join(t.TempDir(), "keep.txt")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	logger := testLogger{t}
	_ = cleanupDirectory(root, time.Nanosecond, 1, "", logger)
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was affected: %v", err)
	}
}

type testLogger struct{ t *testing.T }

func (l testLogger) Printf(format string, args ...any) { l.t.Logf(format, args...) }

func TestDependentItemOptionsAreEmptyWithoutCategory(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/items?pageSize=8", nil)
	recorder := httptest.NewRecorder()
	handleItems(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Filters struct {
			Subcategories []string `json:"subcategories"`
			Qualities     []string `json:"qualities"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Filters.Subcategories) != 0 || len(response.Filters.Qualities) != 0 {
		t.Fatalf("dependent item options must be empty without category: %#v", response.Filters)
	}
}

func TestDependentMonsterOptionsAreEmptyWithoutCategory(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/monsters?pageSize=8", nil)
	recorder := httptest.NewRecorder()
	handleMonsters(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Filters struct {
			Types []string `json:"types"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Filters.Types) != 0 {
		t.Fatalf("dependent monster options must be empty without category: %#v", response.Filters.Types)
	}
}

func TestItemDropsIncludeMonsterLevel(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	runtime := activeRuntime("kiss")
	entries := runtime.resolved[2300304]
	if len(entries) == 0 {
		t.Fatal("group 2300304 has no items")
	}
	checked := 0
	for _, drop := range itemDropSources(entries[0].ItemID, runtime) {
		if drop.MonsterID == 0 {
			continue
		}
		monster := store.monstersByID[drop.MonsterID]
		if monster == nil {
			t.Fatalf("drop references missing monster %d", drop.MonsterID)
		}
		if drop.MonsterLevel != monster.Level {
			t.Fatalf("wrong monster level for %d: got %d want %d", drop.MonsterID, drop.MonsterLevel, monster.Level)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no monster drops were checked")
	}
}

func TestItemPreviewStatsExcludeWeight(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for i := range store.data.Items {
		for _, stat := range itemStats(&store.data.Items[i]) {
			if stat["name"] == "Вес" {
				t.Fatalf("weight leaked into preview stats for item %d", store.data.Items[i].ID)
			}
		}
	}
}

func TestMonsterPreviewUsesReliableStatsAndExcludesExperience(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for i := range store.data.Monsters {
		monster := &store.data.Monsters[i]
		if monster.HP == 0 || monster.Defense == 0 {
			continue
		}
		summary := monsterSummary(monster)
		stats, ok := summary["stats"].([]map[string]any)
		if !ok {
			t.Fatalf("unexpected stats type: %T", summary["stats"])
		}
		names := map[string]bool{}
		for _, stat := range stats {
			names[stat["name"].(string)] = true
		}
		if !names["HP"] || !names["Физическая защита"] {
			t.Fatalf("monster preview lacks reliable stats: %#v", stats)
		}
		if names["Опыт"] {
			t.Fatalf("deprecated experience value is still shown: %#v", stats)
		}
		return
	}
	t.Fatal("no monster with HP and defense found")
}

func TestItemAndMonsterRecordsRemainUnchanged(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{"items", store.data.Items, "abbc7304ae96ad1e1f7e57ff0e50562092a8bcc9534e7a9e37949429c89b5438"},
		{"monsters", store.data.Monsters, "06f10b2e30feca6fe3d332fcf695e5ca8757096dff43230b5a07906e0e5e0c63"},
	}
	for _, test := range tests {
		encoded, err := json.Marshal(test.value)
		if err != nil {
			t.Fatal(err)
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(encoded))
		if actual != test.expected {
			t.Fatalf("%s changed: got %s want %s", test.name, actual, test.expected)
		}
	}
}

func TestSecurityHeadersRejectForeignHost(t *testing.T) {
	called := false
	handler := withSecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/health", nil)
	request.Host = "example.test"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("foreign host was not rejected: status=%d called=%v", recorder.Code, called)
	}
}

func TestSecurityHeadersSetStrictBrowserProtections(t *testing.T) {
	handler := withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodGet, "http://wails.localhost/", nil)
	request.Host = "wails.localhost"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
	csp := recorder.Header().Get("Content-Security-Policy")
	for _, required := range []string{"default-src 'self'", "script-src 'self'", "style-src 'self'", "connect-src 'self'", "frame-src 'none'", "object-src 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, required) {
			t.Fatalf("CSP missing %q: %s", required, csp)
		}
	}
	if strings.Contains(csp, "'unsafe-inline'") || strings.Contains(csp, "'unsafe-eval'") {
		t.Fatalf("CSP contains unsafe script/style policy: %s", csp)
	}
	if recorder.Header().Get("X-Frame-Options") != "DENY" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("browser hardening headers are incomplete")
	}
}

func TestSecurityHeadersRejectCrossSiteWrite(t *testing.T) {
	called := false
	handler := withSecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest(http.MethodPost, "http://wails.localhost/api/user-data", strings.NewReader(`{"schemaVersion":1}`))
	request.Host = "wails.localhost"
	request.Header.Set("Origin", "https://example.test")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("cross-site write was not rejected: status=%d called=%v", recorder.Code, called)
	}
}

func TestJSONDecoderRequiresApplicationJSONAndSingleValue(t *testing.T) {
	for name, testCase := range map[string]struct {
		contentType string
		body        string
		status      int
	}{
		"wrong content type": {"text/plain", `{}`, http.StatusUnsupportedMediaType},
		"trailing value":     {"application/json", `{} {}`, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", testCase.contentType)
			recorder := httptest.NewRecorder()
			var target struct{}
			if decodeJSONRequest(recorder, request, &target, 4096) {
				t.Fatal("invalid request was accepted")
			}
			if recorder.Code != testCase.status {
				t.Fatalf("status=%d want=%d", recorder.Code, testCase.status)
			}
		})
	}
}

func TestCaptureWriterKeepsFirstStatus(t *testing.T) {
	writer := newCaptureWriter()
	writer.WriteHeader(http.StatusCreated)
	writer.WriteHeader(http.StatusInternalServerError)
	if writer.status != http.StatusCreated {
		t.Fatalf("status=%d want=%d", writer.status, http.StatusCreated)
	}
}

func TestResponseCacheRejectsOversizedKey(t *testing.T) {
	cache := newResponseCache(4, 1<<20, time.Minute)
	key := strings.Repeat("x", maxCacheKeyBytes+1)
	cache.Put(key, http.StatusOK, map[string]string{"Content-Type": "application/json"}, []byte(`{}`))
	if _, ok := cache.Get(key); ok {
		t.Fatal("oversized cache key was stored")
	}
}

func TestResponseCacheRejectsOversizedEntry(t *testing.T) {
	cache := newResponseCache(4, 8<<20, time.Minute)
	body := make([]byte, maxCacheEntryBytes+1)
	cache.Put("large", http.StatusOK, map[string]string{"Content-Type": "application/json"}, body)
	if _, ok := cache.Get("large"); ok {
		t.Fatal("oversized cache entry was stored")
	}
}

func TestCacheableResponseRequestRequiresGET(t *testing.T) {
	get := httptest.NewRequest(http.MethodGet, "/api/monsters/10042?server=kiss", nil)
	if !isCacheableResponseRequest(get) {
		t.Fatal("monster detail GET should be cacheable")
	}
	post := httptest.NewRequest(http.MethodPost, "/api/monsters/10042?server=kiss", nil)
	if isCacheableResponseRequest(post) {
		t.Fatal("non-GET request was marked cacheable")
	}
}

func TestHealthIncludesApplicationMarker(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recorder := httptest.NewRecorder()
	app := &application{}
	app.handleHealth(recorder, request)
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["application"] != applicationID || response["status"] != "ok" {
		t.Fatalf("unexpected health response: %#v", response)
	}
}

func TestProfileReadRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	data := make([]byte, maxProfileBytes+1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfileFile(path); err == nil {
		t.Fatal("oversized profile was accepted")
	}
}

func TestInterfaceVersionMatchesApplication(t *testing.T) {
	data, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Iris Online") || !strings.Contains(string(script), "const APP_VERSION = '"+appVersion+"'") {
		t.Fatalf("interface version does not match %s", appVersion)
	}
}

func TestNavigationDoesNotDuplicateItemCategories(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	start := strings.Index(script, "const navItems = [")
	end := strings.Index(script, "const mobileItems =")
	if start < 0 || end <= start {
		t.Fatal("navigation block was not found")
	}
	navigation := script[start:end]
	for _, duplicate := range []string{"['weapons'", "['armor'"} {
		if strings.Contains(navigation, duplicate) {
			t.Fatalf("duplicate top-level item section remains: %s", duplicate)
		}
	}
	for _, duplicate := range []string{"quickCard('weapons'", "quickCard('armor'"} {
		if strings.Contains(script, duplicate) {
			t.Fatalf("duplicate home shortcut remains: %s", duplicate)
		}
	}
}

func TestTooltipCategoriesUseSourcePresentation(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, expected := range []string{
		"if (id === 0 || value.toLocaleLowerCase('ru-RU') === 'не указано') return 'Покупной';",
		"if (id === 9 || value.toLocaleLowerCase('ru-RU').includes('событийн')) return 'Ивентовый';",
		"if (id === 0) return 'quality-shop';",
		"if (id === 9) return 'quality-event';",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("tooltip category presentation marker is missing: %s", expected)
		}
	}
	if strings.Count(script, "qualityBadge(item.quality, item.qualityId)") != 2 {
		t.Fatal("quality badge helper is not used with source tooltip color in both item card and item detail")
	}
}

func TestSourceDatesArePublished(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if store.data.Meta.DataUpdatedAt != "2026-07-07" || store.data.Meta.DropUpdatedAt != "2026-02-27" {
		t.Fatalf("unexpected data dates: %#v", store.data.Meta)
	}
	kiss := store.data.Servers["kiss"]
	if kiss.DirectDropsUpdatedAt != "2025-04-03" || kiss.DropListsUpdatedAt != "2025-04-03" || kiss.WorldDropsUpdatedAt != "2026-02-27" {
		t.Fatalf("unexpected Kiss drop dates: %#v", kiss)
	}
}

func TestDropProbabilitiesAreAlwaysVisibleWithoutHiddenMode(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, expected := range []string{"formatChance(choice.baseSelectionChance)", "Шанс группы", "за одну основную попытку", "Как рассчитывается шанс"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("probability marker is missing: %s", expected)
		}
	}
	for _, forbidden := range []string{"expertMode", "versionClickCount", "expertPasswordDigest", "sha256Hex", "window.prompt", "qualitativeChance"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("obsolete hidden-mode marker remains: %s", forbidden)
		}
	}
	page, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "versionTrigger") || strings.Contains(string(page), `<button class="version-trigger"`) {
		t.Fatal("version is still interactive")
	}
}

func TestDropInterfaceIsConciseAndOpensSlotsToGroupLevel(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, forbidden := range []string{"Сумма предметов", "Без группы", "sourceLine ? ` · строка", "qualitativeChance", "Ячейка выпадения"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("obsolete drop label remains: %s", forbidden)
		}
	}
	for _, expected := range []string{"Актуальность данных", "data-monster-drops-host", "data-drop-group", "Шанс группы", "Вариант добычи"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("updated drop marker is missing: %s", expected)
		}
	}
	if strings.Contains(script, "<details class=\"drop-choice\" open>") {
		t.Fatal("item lists must remain collapsed until the player opens a group")
	}
	if !strings.Contains(script, "Список загрузится после открытия раздела.") {
		t.Fatal("monster drop accordion is not lazy")
	}
}

func TestDropListSourceOrderIsNotSorted(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	entries := store.data.Servers["kiss"].DropLists["100601"]
	want := []int{80017, 80020, 80023}
	if len(entries) != len(want) {
		t.Fatalf("group 100601 entries=%d want=%d", len(entries), len(want))
	}
	for index, itemID := range want {
		if entries[index].ItemID != itemID {
			t.Fatalf("group 100601 position %d=%d want=%d", index+1, entries[index].ItemID, itemID)
		}
	}
}

func TestWorldRulePreservesFourAlternatives(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, rule := range store.data.Servers["kiss"].WorldRules {
		if rule.SourceLine != 140 {
			continue
		}
		want := []int{9999905, 9999906, 9999907, 9999908}
		if len(rule.Groups) != len(want) {
			t.Fatalf("world rule alternatives=%d want=%d", len(rule.Groups), len(want))
		}
		for index, groupID := range want {
			if rule.Groups[index].GroupID != groupID {
				t.Fatalf("world rule position %d=%d want=%d", index+1, rule.Groups[index].GroupID, groupID)
			}
		}
		return
	}
	t.Fatal("world rule source line 140 was not found")
}

func TestMonsterDropViewContainsOnlyOrdinaryDrop(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	monster := store.monstersByID[10042]
	groups, slots := monsterDropView(monster, activeRuntime("kiss"))
	if len(slots) == 0 {
		t.Fatal("ordinary drop slots are missing")
	}
	for _, slot := range slots {
		if slot.Source != "Выпадение монстра" {
			t.Fatalf("monster view exposes non-ordinary source: %#v", slot)
		}
	}
	for _, group := range groups {
		if group.Source != "Выпадение монстра" {
			t.Fatalf("monster view exposes non-ordinary group: %#v", group)
		}
	}
}

func TestWorldDropSourcesAreRuleBasedInsteadOfExpandedPerMonster(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	runtime := activeRuntime("kiss")
	var selected WorldRule
	var itemID int
	for _, rule := range runtime.server.WorldRules {
		for _, group := range rule.Groups {
			items := runtime.resolved[group.GroupID]
			if len(items) == 0 {
				continue
			}
			selected = rule
			itemID = items[0].ItemID
			break
		}
		if itemID != 0 {
			break
		}
	}
	if itemID == 0 {
		t.Fatal("no world-drop item was found")
	}
	found := 0
	for _, drop := range itemDropSources(itemID, runtime) {
		if drop.Source != "Мировое выпадение" || drop.SourceLine != selected.SourceLine {
			continue
		}
		found++
		if drop.MonsterID != 0 || drop.MonsterLevel != 0 {
			t.Fatalf("world rule was incorrectly assigned to a concrete monster: %#v", drop)
		}
		if drop.Context != worldRuleContext(selected) {
			t.Fatalf("world rule context=%q want=%q", drop.Context, worldRuleContext(selected))
		}
	}
	if found == 0 {
		t.Fatal("selected world rule was not exposed as an item source")
	}
}

func TestWorldRuleContextUsesLocationLevelAndMonsterType(t *testing.T) {
	tests := []struct {
		rule WorldRule
		want string
	}{
		{WorldRule{MinLevel: 5, MaxLevel: 15, ContextID: 1, MonsterType: 0}, "Открытая локация · уровни 5–15 · любой тип монстра"},
		{WorldRule{MinLevel: 1, MaxLevel: 100, ContextID: 2, MonsterType: 13}, "Инстанс · уровни 1–100 · только боссы"},
		{WorldRule{MinLevel: 1, MaxLevel: 100, ContextID: 2, MonsterType: 14}, "Инстанс · уровни 1–100 · только рейдовые боссы"},
	}
	for _, test := range tests {
		if got := worldRuleContext(test.rule); got != test.want {
			t.Fatalf("context=%q want=%q", got, test.want)
		}
	}
}

func TestMonsterCanBeFoundByID(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/monsters?q=10042&pageSize=8", nil)
	recorder := httptest.NewRecorder()
	handleMonsters(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Monsters []struct {
			ID int `json:"id"`
		} `json:"monsters"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Monsters) != 1 || response.Monsters[0].ID != 10042 {
		t.Fatalf("unexpected ID search result: %#v", response.Monsters)
	}
}

func TestPaginationHasFirstAndLastPageControls(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	start := strings.Index(script, "  function pagination(")
	end := strings.Index(script[start:], "  function activeFilterCount(")
	if start < 0 || end < 0 {
		t.Fatal("pagination source block was not found")
	}
	block := script[start : start+end]
	for _, want := range []string{
		`aria-label="Первая страница"`,
		`aria-label="Предыдущая страница"`,
		`aria-label="Следующая страница"`,
		`aria-label="Последняя страница"`,
		`${attribute}="1"`,
		`${attribute}="${pages}"`,
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("pagination is missing %q", want)
		}
	}
}

func TestMonsterInterfaceHidesPreviewIDAndKeepsTechnicalID(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	start := strings.Index(script, "  function monsterRow(")
	end := strings.Index(script[start:], "  function pagination(")
	if start < 0 || end < 0 {
		t.Fatal("monsterRow source block was not found")
	}
	rowSource := script[start : start+end]
	if strings.Contains(rowSource, "<span>ID ") {
		t.Fatal("monster ID is still visible in the catalog preview")
	}
	if !strings.Contains(script, "['ID монстра', formatNumber(monster.id)]") {
		t.Fatal("monster ID is missing from technical details")
	}
	if strings.Contains(script, "['Опыт', formatNumber(monster.exp)]") {
		t.Fatal("deprecated monster experience is still displayed")
	}
}

func TestDetailPagesRenderPropertiesOnceAndSkipEmptyDescriptions(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, forbidden := range []string{"accordion('Все характеристики'", "accordion('Дополнительные эффекты'", "Описание отсутствует."} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("obsolete or empty detail UI remains: %s", forbidden)
		}
	}
	properties := strings.Index(script, "gameProperties(presentation, recipeContext ? 'Характеристики рецепта' : 'Характеристики предмета',")
	bestSource := strings.Index(script, "<span class=\"eyebrow\">Лучший источник</span>")
	if properties < 0 || bestSource < 0 || properties > bestSource {
		t.Fatal("item properties are not rendered before the best source")
	}
	for _, required := range []string{"itemClassBadge(presentation.classes)", "property-group--base", "property-group--bonus", "property-group--price", "cardSlotsRow(presentation.cardSlots)", "card-slot-chip", "recipeProductHTML(data.recipeProduct, item)", "recipeProductEffectText", "Что даёт готовый предмет"} {
		if !strings.Contains(script, required) {
			t.Fatalf("new item presentation is missing %q", required)
		}
	}
	if strings.Contains(script, "detail-properties") {
		t.Fatal("obsolete tabular detail-properties layout remains")
	}
	if !strings.Contains(script, "description ? accordion('Описание'") {
		t.Fatal("meaningful descriptions are not rendered conditionally")
	}
}

func TestRecipeProductEffectSuppressesRepeatedProductName(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, required := range []string{
		"function recipeProductEffectText(value, productName)",
		"recipeProductEffectText(product.abilityDescription, product.name)",
		"recipeProductEffectText(fallbackEffect, product.name)",
		"!== normalizedName",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("recipe product duplicate-name suppression is missing %q", required)
		}
	}
}

func TestItemSearchMatchesRussianWordForms(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/search?q=%D0%B3%D0%BD%D0%B5%D0%B2%20%D0%BF%D1%80%D0%B5%D0%B4%D0%BA%D0%BE%D0%B2",
		"/api/items?q=%D0%B3%D0%BD%D0%B5%D0%B2%20%D0%BF%D1%80%D0%B5%D0%B4%D0%BA%D0%BE%D0%B2&pageSize=8",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		if strings.HasPrefix(path, "/api/search") {
			handleSearch(recorder, request)
		} else {
			handleItems(recorder, request)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status=%d: %s", path, recorder.Code, recorder.Body.String())
		}
		var response struct {
			Items []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range response.Items {
			if item.ID == 80592 && item.Name == "Шлем гнева предков" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s did not find an inflected item name: %#v", path, response.Items)
		}
	}
}

func TestSetItemsAreMarkedAndHaveOneClickNavigation(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	summary := itemSummary(store.itemsByID[80592])
	if got, ok := summary["setSize"].(int); !ok || got != 5 {
		t.Fatalf("setSize=%#v want=5", summary["setSize"])
	}
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, expected := range []string{
		"Комплект · ${formatCount(count, 'предмет', 'предмета', 'предметов')}",
		"Предметы комплекта",
		"set-member-link",
		"aria-current=\"page\"",
		"item-inline-set",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("set interface marker is missing: %s", expected)
		}
	}
}

func TestSetSupplementMergedWithoutLoss(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	raw, err := embedded.ReadFile("assets/set_effects.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	var supplement itemSetSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		t.Fatal(err)
	}
	for id, source := range supplement.Sets {
		merged, ok := store.data.ItemSets[id]
		if !ok {
			t.Fatalf("set %s from supplement is missing after merge", id)
		}
		if !reflect.DeepEqual(merged.Effects, source.Effects) {
			t.Fatalf("set %s effects changed during merge", id)
		}
	}
}

func TestSetPresentationKeepsFivePieceEffectAndEquipmentOrder(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/items/80567?server=kiss", nil)
	recorder := httptest.NewRecorder()
	handleItem(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Set *ItemSet `json:"set"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Set == nil {
		t.Fatal("set is missing")
	}
	wantOrder := []int{80567, 80568, 80570, 80569, 80571}
	if len(response.Set.Items) != len(wantOrder) {
		t.Fatalf("members=%d want=%d", len(response.Set.Items), len(wantOrder))
	}
	for i, want := range wantOrder {
		if response.Set.Items[i].ItemID != want {
			t.Fatalf("member order[%d]=%d want=%d", i, response.Set.Items[i].ItemID, want)
		}
	}
	foundFive := false
	for _, effect := range response.Set.Effects {
		if effect.Required == 5 && effect.Active != nil && effect.Active.ID == 62021 {
			foundFive = true
		}
	}
	if !foundFive {
		t.Fatal("5-piece active set effect 62021 is missing from item API")
	}
}

func TestItemCanBeFoundByID(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/items?q=80592&pageSize=8", nil)
	recorder := httptest.NewRecorder()
	handleItems(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != 80592 {
		t.Fatalf("unexpected ID search result: %#v", response.Items)
	}
}

func TestPrimaryNavigationContainsOnlyWorkingSections(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	start := strings.Index(script, "const navItems = [")
	end := -1
	if start >= 0 {
		end = strings.Index(script[start:], "];")
	}
	if start < 0 || end < 0 {
		t.Fatal("primary navigation definition is missing")
	}
	navigation := script[start : start+end]
	for _, route := range []string{"items", "monsters", "favorites"} {
		if !strings.Contains(navigation, `route: '`+route+`'`) {
			t.Fatalf("working route %q is missing", route)
		}
	}
	for _, forbidden := range []string{"weapons", "armor", "craft", "maps", "quests"} {
		if strings.Contains(navigation, `route: '`+forbidden+`'`) {
			t.Fatalf("placeholder route %q is exposed", forbidden)
		}
	}
	index, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `href="https://irisonline.ru/"`) {
		t.Fatal("official Iris Online link is missing")
	}
}

func TestResultRowsUseSeparateLinkAndFavoriteButton(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, expected := range []string{`<article class="result-row">`, `<a class="result-main"`, `<button class="favorite-button`} {
		if !strings.Contains(script, expected) {
			t.Fatalf("accessible result-row marker is missing: %s", expected)
		}
	}
	for _, forbidden := range []string{`<article role="link"`, `data-open=`} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("nested/clickable article pattern remains: %s", forbidden)
		}
	}
}

func TestRarityColorsArePreserved(t *testing.T) {
	data, err := os.ReadFile("web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	styles := string(data)
	for className, color := range map[string]string{
		"quality-unique": "#fff600",
		"quality-epic":   "#d800ff",
		"quality-rare":   "#00fffc",
		"quality-normal": "#ffffff",
		"quality-magic":  "#00ff00",
		"quality-shop":   "#ffcd00",
		"quality-event":  "#c9a0dc",
	} {
		marker := ".rarity-label." + className + " { color: " + color
		if !strings.Contains(styles, marker) {
			t.Fatalf("rarity color changed or missing: %s", marker)
		}
	}
}

func TestCatalogSearchUsesPartialRefreshAndListDefault(t *testing.T) {
	scriptData, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(scriptData)
	for _, expected := range []string{"async function refreshCatalog", "[data-catalog-results]", "catalogDebounce = setTimeout(() => refreshCatalog(), SEARCH_DEBOUNCE)"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("partial catalog refresh marker is missing: %s", expected)
		}
	}
	profile := defaultProfile()
	if profile.Settings.View != "list" {
		t.Fatalf("default catalog view=%q want=list", profile.Settings.View)
	}
}

func TestFavoritesPaginationKeepsMoreThanFiveHundredEntries(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if len(store.data.Items) < 620 {
		t.Fatalf("test needs at least 620 items, got %d", len(store.data.Items))
	}
	keys := make([]string, 0, 620)
	for index := 0; index < 620; index++ {
		keys = append(keys, fmt.Sprintf("item:%d", store.data.Items[index].ID))
	}
	body, _ := json.Marshal(map[string]any{"keys": keys, "server": "kiss", "page": 13, "pageSize": 50})
	request := httptest.NewRequest(http.MethodPost, "/api/favorites", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handleFavorites(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Rows      []map[string]any `json:"rows"`
		Total     int              `json:"total"`
		Page      int              `json:"page"`
		Pages     int              `json:"pages"`
		TotalKeys int              `json:"totalKeys"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Total != 620 || response.TotalKeys != 620 || response.Page != 13 || response.Pages != 13 || len(response.Rows) != 20 {
		t.Fatalf("unexpected paginated favorites response: total=%d totalKeys=%d page=%d pages=%d rows=%d", response.Total, response.TotalKeys, response.Page, response.Pages, len(response.Rows))
	}
}

func TestLegacyProfileRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	backupDir := filepath.Join(dir, "Backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "schemaVersion": 1,
  "updatedAt": "2025-01-01T00:00:00Z",
  "migrated": true,
  "settings": {"server": "kiss", "theme": "dark", "view": "cards"},
  "itemFilters": {"q": "гнев предков", "sort": "name"},
  "monsterFilters": {"sort": "level"},
  "favorites": ["item:77", "monster:10042"],
  "history": ["гнев предков"]
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: path, Backups: backupDir})
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Get()
	if profile.Settings.View != "cards" || len(profile.Favorites) != 2 {
		t.Fatalf("legacy profile was not preserved: %#v", profile)
	}
	if len(profile.ItemFilters) != 0 || len(profile.MonsterFilters) != 0 {
		t.Fatalf("legacy catalog filters were not cleared: item=%#v monster=%#v", profile.ItemFilters, profile.MonsterFilters)
	}
	profile.Settings.Theme = "light"
	if err := store.Replace(profile); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.SchemaVersion != 1 || reloaded.Settings.View != "cards" || reloaded.Settings.Theme != "light" || len(reloaded.Favorites) != 2 {
		t.Fatalf("round-tripped profile is incompatible: %#v", reloaded)
	}
}

func TestRecentlyViewedSanitizationAndProfileReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	backupDir := filepath.Join(dir, "Backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	profile := defaultProfile()
	profile.RecentlyViewed = []recentViewEntry{
		{Type: "monster", ID: 253, Name: "Монстр 253", Meta: "Босс · Уровень 25 · ID 253"},
		{Type: "item", ID: 253, Name: "Предмет 253"},
		{Type: "recipe", ID: 253, Name: "Рецепт 253"},
		{Type: "monster", ID: 253, Name: "Дубликат"},
		{Type: "unknown", ID: 10, Name: "Лишнее"},
		{Type: "item", ID: -1, Name: "Неверный ID"},
		{Type: "item", ID: 254, Name: ""},
	}
	store := &profileStore{path: path, backup: filepath.Join(backupDir, "profile.json.bak"), profile: defaultProfile()}
	if err := store.Replace(profile); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadProfileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.RecentlyViewed) != 3 {
		t.Fatalf("recently viewed entries=%d want=3: %#v", len(reloaded.RecentlyViewed), reloaded.RecentlyViewed)
	}
	if reloaded.RecentlyViewed[0].Type != "monster" || reloaded.RecentlyViewed[0].ID != 253 || reloaded.RecentlyViewed[1].Type != "item" || reloaded.RecentlyViewed[1].ID != 253 || reloaded.RecentlyViewed[2].Type != "recipe" || reloaded.RecentlyViewed[2].ID != 253 {
		t.Fatalf("recent-view types with same numeric ID were not preserved independently: %#v", reloaded.RecentlyViewed)
	}
	if reloaded.RecentlyViewed[0].Meta != "Босс · Уровень 25 · ID 253" {
		t.Fatalf("recent-view metadata was not preserved: %#v", reloaded.RecentlyViewed[0])
	}
}

func TestLegacyProfileWithoutRecentlyViewedRemainsCompatible(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	backupDir := filepath.Join(dir, "Backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"schemaVersion":1,"updatedAt":"2025-01-01T00:00:00Z","migrated":true,"settings":{"server":"kiss","theme":"dark","view":"list"},"itemFilters":{},"monsterFilters":{},"favorites":[],"history":[]}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := newProfileStore(appPaths{Profile: path, Backups: backupDir})
	if err != nil {
		t.Fatal(err)
	}
	profile := store.Get()
	if profile.RecentlyViewed == nil || len(profile.RecentlyViewed) != 0 {
		t.Fatalf("legacy recently viewed should load as an empty list: %#v", profile.RecentlyViewed)
	}
}

func TestEmbeddedGameDatabaseMatchesConfirmedVersion(t *testing.T) {
	data, err := os.ReadFile("assets/game_data.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	const want = "7c3698494233696f2f5728ef17f7e13953159191f966d77b90742dbced23875e"
	if got != want {
		t.Fatalf("embedded game database changed: got %s want %s", got, want)
	}
}

func TestDesktopModeHasNoLoopbackListenerOrBrowserLaunch(t *testing.T) {
	serverData, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	desktopData, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	serverSource := string(serverData)
	desktopSource := string(desktopData)
	for _, forbidden := range []string{"net.Listen(", "openBrowser(", "127.0.0.1:8765", "probeExistingInstance("} {
		if strings.Contains(serverSource, forbidden) || strings.Contains(desktopSource, forbidden) {
			t.Fatalf("desktop production path still contains legacy browser/listener marker %q", forbidden)
		}
	}
	for _, required := range []string{"wails.Run(", "SingleInstanceLock:", "WebviewUserDataPath:", "Assets:     webAssets"} {
		if !strings.Contains(desktopSource, required) {
			t.Fatalf("desktop production marker is missing: %s", required)
		}
	}
}

func TestSearchStartsEmptyAndRecentlyViewedCanBeCleared(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, marker := range []string{
		"resetTransientCatalogFilters()",
		"state.itemFilters = defaultItemFilters()",
		"state.monsterFilters = defaultMonsterFilters()",
		"localStorage.removeItem('iris-item-filters')",
		"localStorage.removeItem('iris-monster-filters')",
		"itemFilters: {}",
		"monsterFilters: {}",
		"globalSearch.value = ''",
		"data-action=\"clear-recently-viewed\"",
		"function clearRecentlyViewed()",
		"knownSource: ''",
		"Известно, где получить",
		"<h2 id=\"serverDifferenceTitle\">Сервер</h2>",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("startup-search/recent-view marker is missing: %s", marker)
		}
	}
}

func TestPlayerFacingDropHelpHidesInternalGameFileDetails(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ToLower(string(data))
	for _, forbidden := range []string{"item_change", "накопительным весам", "вес в исходной таблице", ".txt"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("internal/technical drop detail remains in player-facing UI: %s", forbidden)
		}
	}
	for _, expected := range []string{"шанс группы", "если группа выбрана", "оба выбора сработают подряд", "шанс при открытии"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("simple drop explanation marker is missing: %s", expected)
		}
	}
}

func TestQuestSourceNameIsNotRepeatedInDetails(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	if strings.Contains(script, "isQuest ? [drop.quest, drop.context]") {
		t.Fatal("quest source still repeats drop.quest in both title and details")
	}
	if !strings.Contains(script, "isQuest ? [drop.context]") {
		t.Fatal("quest source details are not separated from the source title")
	}
}

func TestMorePopoverUsesNativeTabOrderInsteadOfPartialMenuARIA(t *testing.T) {
	page, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	if strings.Contains(html, `role="menu"`) || strings.Contains(html, `role="menuitem"`) {
		t.Fatal("partial ARIA menu semantics remain")
	}
	if !strings.Contains(html, `id="moreMenu"`) || !strings.Contains(html, `id="moreButton"`) {
		t.Fatal("more popover controls are missing")
	}
}

func TestCompleteSetSupplementIsAvailableThroughStore(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	raw, err := embedded.ReadFile("assets/set_effects.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	var supplement itemSetSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		t.Fatal(err)
	}
	if got, want := len(supplement.Sets), 458; got != want {
		t.Fatalf("supplement sets=%d want=%d", got, want)
	}
	rows := 0
	thresholds := map[int]int{}
	activeAtFive := 0
	for key, expected := range supplement.Sets {
		actual, ok := store.data.ItemSets[key]
		if !ok {
			t.Fatalf("set %s is missing after merge", key)
		}
		if actual.Name != expected.Name {
			t.Fatalf("set %s name=%q want=%q", key, actual.Name, expected.Name)
		}
		expectedJSON, _ := json.Marshal(expected.Effects)
		actualJSON, _ := json.Marshal(actual.Effects)
		if !bytes.Equal(actualJSON, expectedJSON) {
			t.Fatalf("set %s effects differ after merge\nactual=%s\nwant=%s", key, actualJSON, expectedJSON)
		}
		for _, effect := range actual.Effects {
			rows++
			thresholds[effect.Required]++
			if effect.Required == 5 && effect.Active != nil {
				activeAtFive++
			}
		}
	}
	if rows != 972 {
		t.Fatalf("set effect rows=%d want=972", rows)
	}
	for _, threshold := range []int{2, 3, 4, 5} {
		if thresholds[threshold] == 0 {
			t.Fatalf("threshold %d was lost: %#v", threshold, thresholds)
		}
	}
	if len(thresholds) != 4 {
		t.Fatalf("unexpected thresholds: %#v", thresholds)
	}
	if activeAtFive != 122 {
		t.Fatalf("active five-piece effects=%d want=122", activeAtFive)
	}
}

func TestKnownPreviouslyLostSetEffectsAreRestored(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		setID    string
		required int
		activeID int
	}{
		{"80592", 5, 62026},
		{"80145", 4, 62006},
	}
	for _, tc := range cases {
		set := store.data.ItemSets[tc.setID]
		found := false
		for _, effect := range set.Effects {
			if effect.Required == tc.required && effect.Active != nil && effect.Active.ID == tc.activeID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("set %s lost required=%d active=%d", tc.setID, tc.required, tc.activeID)
		}
	}
}

func TestSetEffectOrderAndMultipleRowsPerThresholdArePreserved(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	set := store.data.ItemSets["101656"]
	got := make([]int, 0, len(set.Effects))
	for _, effect := range set.Effects {
		got = append(got, effect.Required)
	}
	want := []int{2, 3, 4, 5, 2, 3, 4, 5}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("set source order=%v want=%v", got, want)
	}
	wedding := store.data.ItemSets["1112080"]
	rowsAtTwo := 0
	linesAtTwo := 0
	for _, effect := range wedding.Effects {
		if effect.Required == 2 {
			rowsAtTwo++
			linesAtTwo += len(effect.Options)
			if effect.Active != nil {
				linesAtTwo++
			}
		}
	}
	if rowsAtTwo != 2 || linesAtTwo != 3 {
		t.Fatalf("multiple same-threshold rows lost: rows=%d lines=%d", rowsAtTwo, linesAtTwo)
	}
}

func TestItemAPIKeepsExplicitZeroOption(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items/22058?server=kiss", nil)
	recorder := httptest.NewRecorder()
	handleItem(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Item    Item `json:"item"`
		Bonuses []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"bonuses"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	zeroSource := false
	for _, option := range response.Item.Options {
		if option.Value == 0 {
			zeroSource = true
			break
		}
	}
	if !zeroSource {
		t.Fatalf("explicit zero source option was lost: %#v", response.Item.Options)
	}
	found := false
	for _, bonus := range response.Bonuses {
		if bonus.Value == "+0" || bonus.Value == "+0.00%" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("zero option is missing from presentation data: %#v", response.Bonuses)
	}
}

func TestMonsterExperienceRemainsTechnicalOnly(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	presentationStart := strings.Index(script, "function monsterPresentation")
	presentationEnd := strings.Index(script[presentationStart:], "function itemTechnicalRows")
	if presentationStart < 0 || presentationEnd < 0 {
		t.Fatal("monster presentation block not found")
	}
	presentation := script[presentationStart : presentationStart+presentationEnd]
	if strings.Contains(presentation, "monster.exp") {
		t.Fatal("obsolete monster EXP is exposed as a player-facing stat")
	}
	technicalStart := strings.Index(script, "function monsterTechnicalRows")
	technicalEnd := strings.Index(script[technicalStart:], "function itemClassBadge")
	technical := script[technicalStart : technicalStart+technicalEnd]
	if !strings.Contains(technical, "monster.exp") {
		t.Fatal("raw monster EXP is not retained in technical details")
	}
}

func TestItemAbilitySupplementRestoresOnlyMissingProjectionFields(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	item := store.itemsByID[130010000]
	if item == nil {
		t.Fatal("expected item 130010000")
	}
	if item.PhysicalDefense != 511 || item.MagicDefense != 538 {
		t.Fatalf("restored defenses=%d/%d want=511/538", item.PhysicalDefense, item.MagicDefense)
	}
	if len(item.Options) != 4 {
		t.Fatalf("restored options=%#v", item.Options)
	}

	conflict := store.itemsByID[151201001]
	if conflict == nil || len(conflict.Options) != 1 || conflict.Options[0].Type != 120 || conflict.Options[0].Value != 52 {
		t.Fatalf("embedded conflicting option was overwritten: %#v", conflict)
	}
}

func TestAllItemAbilitySupplementRowsMergeIntoKnownItems(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	raw, err := embedded.ReadFile("assets/item_abilities.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	var supplement itemAbilitySupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		t.Fatal(err)
	}
	if got, want := len(supplement.Items), 13927; got != want {
		t.Fatalf("ability supplement items=%d want=%d", got, want)
	}
	for key, patch := range supplement.Items {
		id, err := strconv.Atoi(key)
		if err != nil {
			t.Fatal(err)
		}
		item := store.itemsByID[id]
		if item == nil {
			t.Fatalf("supplement item %d missing", id)
		}
		if patch.PhysicalDefense != nil && item.PhysicalDefense != *patch.PhysicalDefense {
			t.Fatalf("item %d physicalDefense=%d want=%d", id, item.PhysicalDefense, *patch.PhysicalDefense)
		}
		if patch.MagicDefense != nil && item.MagicDefense != *patch.MagicDefense {
			t.Fatalf("item %d magicDefense=%d want=%d", id, item.MagicDefense, *patch.MagicDefense)
		}
		if patch.AttackRange != nil && item.AttackRange != *patch.AttackRange {
			t.Fatalf("item %d attackRange=%d want=%d", id, item.AttackRange, *patch.AttackRange)
		}
		if patch.Cooldown != nil && item.Cooldown != *patch.Cooldown {
			t.Fatalf("item %d cooldown=%d want=%d", id, item.Cooldown, *patch.Cooldown)
		}
		if patch.Options != nil {
			want, _ := json.Marshal(*patch.Options)
			got, _ := json.Marshal(item.Options)
			if !bytes.Equal(got, want) {
				t.Fatalf("item %d options=%s want=%s", id, got, want)
			}
		}
		if item.AbilityDescription != patch.AbilityDescription || item.AbilityDescriptionIndex != patch.AbilityDescriptionIndex {
			t.Fatalf("item %d ability description projection mismatch", id)
		}
		if item.UseMapType != patch.UseMapType || item.MakeSkill != patch.MakeSkill || item.MakeSkillExp != patch.MakeSkillExp || item.GuildUse != patch.GuildUse {
			t.Fatalf("item %d limit projection mismatch", id)
		}
	}
}

func TestRestoredAbilityDescriptionAndLimitReachItemAPI(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	item := store.itemsByID[800001]
	if item == nil || item.AbilityDescription != "Восстанавливает 100 ОЗ." {
		t.Fatalf("restored ability description missing: %#v", item)
	}
	profession := store.itemsByID[871017]
	if profession == nil || profession.MakeSkill != 1 || profession.MakeSkillExp != 70 {
		t.Fatalf("profession restriction missing: %#v", profession)
	}
}

func TestSecurityHeadersRejectCrossSiteRead(t *testing.T) {
	called := false
	handler := withSecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	request := httptest.NewRequest(http.MethodGet, "http://wails.localhost/api/meta", nil)
	request.Host = "wails.localhost"
	request.Header.Set("Origin", "https://example.test")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("cross-site read was not rejected: status=%d called=%v", recorder.Code, called)
	}
}

func TestJSONDecoderRejectsDuplicateFields(t *testing.T) {
	for _, body := range []string{`{"id":"a","id":"b"}`, `{"outer":{"x":1,"x":2}}`} {
		request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		var target map[string]any
		if decodeJSONRequest(recorder, request, &target, 4096) {
			t.Fatalf("duplicate JSON was accepted: %s", body)
		}
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("duplicate JSON status=%d want=%d", recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestJSONDecoderRejectsOversizeAndUnknownFields(t *testing.T) {
	t.Run("oversize", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(strings.Repeat("x", 128)))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		var target map[string]any
		if decodeJSONRequest(recorder, request, &target, 32) || recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized JSON status=%d", recorder.Code)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"id":"ok","unexpected":true}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		var target struct {
			ID string `json:"id"`
		}
		if decodeJSONRequest(recorder, request, &target, 4096) || recorder.Code != http.StatusBadRequest {
			t.Fatalf("unknown JSON field status=%d", recorder.Code)
		}
	})
}

func TestAPIConcurrencyLimitRejectsOverflow(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := withAPIConcurrencyLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}), 1)

	firstDone := make(chan int, 1)
	go func() {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
		firstDone <- recorder.Code
	}()
	<-entered
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("concurrency overflow status=%d", second.Code)
	}
	close(release)
	if code := <-firstDone; code != http.StatusNoContent {
		t.Fatalf("first request status=%d", code)
	}
}

func TestStaticRouteCannotTraverseEmbeddedFS(t *testing.T) {
	app := &application{cache: newResponseCache(4, 1<<20, time.Minute), ctx: context.Background()}
	handler := app.routes()
	request := httptest.NewRequest(http.MethodGet, "http://wails.localhost/../../../../etc/passwd", nil)
	request.Host = "wails.localhost"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if strings.Contains(recorder.Body.String(), "root:x:") {
		t.Fatal("path traversal exposed host filesystem")
	}
}

func TestLegacyProfilePreservesSafeUnknownTopLevelData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	backupDir := filepath.Join(dir, "Backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "schemaVersion": 1,
  "updatedAt": "2026-01-01T00:00:00Z",
  "migrated": true,
  "settings": {"server": "original", "theme": "light", "view": "list"},
  "itemFilters": {"q": "эфир", "sort": "name"},
  "monsterFilters": {"category": "Монстр", "type": "boss", "sort": "level"},
  "favorites": ["item:77", "monster:10042"],
  "history": ["эфир"],
  "futureSafe": {"keep": true, "value": 17}
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	profileStore, err := newProfileStore(appPaths{Profile: path, Backups: backupDir})
	if err != nil {
		t.Fatal(err)
	}
	profile := profileStore.Get()
	if profile.Settings.Server != "original" || profile.Settings.Theme != "light" || profile.Settings.View != "list" || len(profile.Favorites) != 2 || len(profile.History) != 1 {
		t.Fatalf("legacy persistent profile values were not preserved: %#v", profile)
	}
	if len(profile.ItemFilters) != 0 || len(profile.MonsterFilters) != 0 {
		t.Fatalf("catalog filters must not survive application restart: item=%#v monster=%#v", profile.ItemFilters, profile.MonsterFilters)
	}
	profile.Settings.Theme = "dark"
	if err := profileStore.Replace(profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var future map[string]any
	if err := json.Unmarshal(raw["futureSafe"], &future); err != nil || future["keep"] != true || future["value"] != float64(17) {
		t.Fatalf("safe unknown profile data was lost: %s", raw["futureSafe"])
	}
}

func TestProfileUnknownFieldAfterOversizedKeyIsPreserved(t *testing.T) {
	extra := map[string]json.RawMessage{
		strings.Repeat("a", 81): json.RawMessage(`{"discard":true}`),
		"zFutureSafe":           json.RawMessage(`{"keep":true}`),
	}

	sanitized := sanitizeProfileExtra(extra)
	if _, ok := sanitized["zFutureSafe"]; !ok {
		t.Fatal("safe unknown profile field after oversized key was dropped")
	}
	if _, ok := sanitized[strings.Repeat("a", 81)]; ok {
		t.Fatal("oversized unknown profile key must be discarded")
	}
}

func TestDesktopFrontendPersistenceHasNoBrowserHeartbeat(t *testing.T) {
	data, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, marker := range []string{
		"function prepareForWindowClose()",
		"persistPendingProfile()",
		"saveProfileBestEffort()",
		"window.addEventListener('beforeunload', prepareForWindowClose)",
		"window.addEventListener('pagehide', prepareForWindowClose)",
		"await loadUserProfile()",
		"fetch('/api/user-data'",
		"keepalive: true",
		"profileController?.abort()",
		"state.suggestionController?.abort()",
		"clearTimeout(profileTimer)",
		"data-monster-drops-host",
		"renderMonsterDropShell()",
		"renderMonsterDropGroup(groupIndex, showAll = false)",
		"DROP_BATCH",
		"data-drop-more",
		"data-drop-all",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("desktop persistence/lazy marker is missing: %s", marker)
		}
	}
	for _, forbidden := range []string{
		"/api/session/",
		"openApplicationSession",
		"heartbeatTimer",
		"sessionCloseSent",
		"navigator.sendBeacon",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("legacy browser lifecycle marker remains: %s", forbidden)
		}
	}
	if strings.Contains(script, "keys.slice(0, 500)") {
		t.Fatal("favorites are still silently truncated to 500 entries")
	}
}

func TestDesktopShutdownIsIdempotentAndFlushesProfile(t *testing.T) {
	dir := t.TempDir()
	paths := appPaths{
		Profile: filepath.Join(dir, "UserData", "profile.json"),
		Backups: filepath.Join(dir, "UserData", "Backups"),
	}
	if err := os.MkdirAll(paths.Backups, 0o700); err != nil {
		t.Fatal(err)
	}
	profiles, err := newProfileStore(paths)
	if err != nil {
		t.Fatal(err)
	}
	profile := defaultProfile()
	profile.Favorites = []string{"item:77"}
	if err := profiles.Replace(profile); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer
	app := &application{
		profile: profiles,
		cache:   newResponseCache(4, 1<<20, time.Minute),
		logger:  log.New(&logs, "", 0),
		ctx:     ctx,
		cancel:  cancel,
	}
	if err := app.shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := app.shutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("shutdown did not cancel application context")
	}
	if count := strings.Count(logs.String(), "запрошено завершение"); count != 1 {
		t.Fatalf("shutdown request count=%d want=1", count)
	}
	loaded, err := loadProfileFile(paths.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Favorites) != 1 || loaded.Favorites[0] != "item:77" {
		t.Fatalf("profile was not flushed: %#v", loaded.Favorites)
	}
}

func TestRecipeUsedSkillsIgnoreUnpublishedMaterialItems(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if _, exists := store.itemsByID[830056]; exists {
		t.Fatal("test fixture 830056 unexpectedly exists in the published item database")
	}
	if _, exists := store.itemUsedSkills[830056]; exists {
		t.Fatal("used-skill index retained an item that is absent from the published database")
	}
	if got := store.itemUsedSkills[81407]; !reflect.DeepEqual(got, []int{4}) {
		t.Fatalf("used skills for item 81407=%v want=[4]", got)
	}

	materials := itemRecipeMaterials(893006)
	if len(materials) == 0 {
		t.Fatal("recipe 893006 has no materials")
	}
	unknownFound := false
	knownFound := false
	for _, material := range materials {
		switch material.ItemID {
		case 830056:
			unknownFound = true
			if material.Known {
				t.Fatal("missing material 830056 was marked as linkable")
			}
		case 1030046:
			knownFound = true
			if !material.Known {
				t.Fatal("published material 1030046 was marked unavailable")
			}
		}
	}
	if !unknownFound || !knownFound {
		t.Fatalf("recipe 893006 edge-case materials missing: unknown=%v known=%v materials=%#v", unknownFound, knownFound, materials)
	}
}
