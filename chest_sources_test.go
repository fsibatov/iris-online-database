package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func findChestContent(t *testing.T, chest *ChestContents, itemID int) ChestContentItem {
	t.Helper()
	if chest == nil {
		t.Fatal("chest contents are nil")
	}
	for _, item := range chest.Items {
		if item.ItemID == itemID {
			return item
		}
	}
	t.Fatalf("item %d not found in chest %d", itemID, chest.ChestID)
	return ChestContentItem{}
}

func TestChestSupplementLoadsExpectedProfiles(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"kiss", "original"} {
		profiles := store.chestProfiles[server]
		if len(profiles) != 399 {
			t.Fatalf("%s chest profiles=%d want=399", server, len(profiles))
		}
		rows := 0
		for _, profile := range profiles {
			rows += len(profile.Rows)
		}
		if rows != 4944 {
			t.Fatalf("%s chest rows=%d want=4944", server, rows)
		}
	}
}

func TestLabyrinthClothChestSilkHatChance(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"kiss", "original"} {
		chest := chestContents(808094, activeRuntime(server))
		item := findChestContent(t, chest, 101402)
		if !item.ChanceKnown {
			t.Fatalf("%s chest 808094 item 101402 chance unexpectedly unknown", server)
		}
		if math.Abs(item.Chance-15.204) > 1e-12 {
			t.Fatalf("%s chest 808094 item 101402 chance=%0.12f want=15.204", server, item.Chance)
		}
		if len(item.Variants) != 1 || !item.Variants[0].ChanceKnown || math.Abs(item.Variants[0].Chance-15.204) > 1e-12 {
			t.Fatalf("%s item variants=%#v", server, item.Variants)
		}
	}
}

func TestChestDrawCountSelectsDistinctRowsFromTier(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	chest := chestContents(1250004, activeRuntime("kiss"))
	if chest == nil || chest.DrawCount != 2 || len(chest.Items) != 15 {
		t.Fatalf("rune chest=%#v", chest)
	}
	want := 100.0 * 2.0 / 15.0
	for _, item := range chest.Items {
		if !item.ChanceKnown || math.Abs(item.Chance-want) > 1e-10 {
			t.Fatalf("item %d chance=%0.12f want=%0.12f", item.ItemID, item.Chance, want)
		}
	}
}

func TestDuplicateChestItemIsAggregatedWithVariants(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	item := findChestContent(t, chestContents(808067, activeRuntime("kiss")), 865008)
	if !item.ChanceKnown || math.Abs(item.Chance-61.06) > 1e-12 {
		t.Fatalf("aggregated chance=%0.12f want=61.06", item.Chance)
	}
	if len(item.Variants) != 3 {
		t.Fatalf("variants=%d want=3: %#v", len(item.Variants), item.Variants)
	}
	gotQuantities := []int{item.Variants[0].Quantity, item.Variants[1].Quantity, item.Variants[2].Quantity}
	sort.Ints(gotQuantities)
	if gotQuantities[0] != 1 || gotQuantities[1] != 2 || gotQuantities[2] != 3 {
		t.Fatalf("variant quantities=%v want=[1 2 3]", gotQuantities)
	}
}

func TestItemSourcesContainOneAggregatedChestSource(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, source := range itemDropSources(101402, activeRuntime("kiss")) {
		if source.Source == "Сундук" && source.ContainerID == 808094 {
			found++
			if !source.ChanceKnown {
				t.Fatal("known chest source marked as unknown")
			}
			if math.Abs(source.ItemBaseChance-15.204) > 1e-12 {
				t.Fatalf("source chance=%0.12f want=15.204", source.ItemBaseChance)
			}
		}
	}
	if found != 1 {
		t.Fatalf("chest 808094 sources=%d want=1", found)
	}
}

func TestNonCatalogChestContainerIsRetained(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"kiss", "original"} {
		if _, ok := store.chestProfiles[server][873063]; !ok {
			t.Fatalf("%s quest-reward box 873063 was filtered out", server)
		}
		chest := chestContents(873063, activeRuntime(server))
		if chest == nil || len(chest.Items) == 0 {
			t.Fatalf("%s quest-reward box contents missing", server)
		}
	}
}

func TestAmbiguousChestProbabilityKeepsContentsButOmitsChance(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"kiss", "original"} {
		profile := store.chestProfiles[server][873079]
		if chestProbabilityKnown(profile) {
			t.Fatalf("%s anomalous chest 873079 probability unexpectedly treated as known", server)
		}
		chest := chestContents(873079, activeRuntime(server))
		if chest == nil || len(chest.Items) == 0 {
			t.Fatalf("%s anomalous chest contents missing", server)
		}
		for _, item := range chest.Items {
			if item.ChanceKnown || item.Chance != 0 {
				t.Fatalf("%s item %d exposes unsupported chance: %#v", server, item.ItemID, item)
			}
			for _, variant := range item.Variants {
				if variant.ChanceKnown || variant.Chance != 0 {
					t.Fatalf("%s item %d variant exposes unsupported chance: %#v", server, item.ItemID, variant)
				}
			}
		}
	}
}

func TestUnknownChestOutputIsVisibleButNotLinkable(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	item := findChestContent(t, chestContents(211112017, activeRuntime("kiss")), 11122017)
	if item.ItemKnown {
		t.Fatalf("unknown output unexpectedly marked as known: %#v", item)
	}
	if !strings.Contains(item.Item, "ID 11122017") {
		t.Fatalf("unknown output did not preserve ID: %q", item.Item)
	}
}

func TestWorldSourceExpansionValidatesExactRulePath(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	rt := activeRuntime("kiss")
	var itemID, sourceLine, groupID, choicePosition, itemPosition int
	for _, rule := range rt.server.WorldRules {
		for choiceIndex, group := range rule.Groups {
			items := rt.resolved[group.GroupID]
			if len(items) == 0 {
				continue
			}
			itemID = items[0].ItemID
			sourceLine = rule.SourceLine
			groupID = group.GroupID
			choicePosition = choiceIndex + 1
			itemPosition = items[0].Position
			break
		}
		if itemID != 0 {
			break
		}
	}
	if itemID == 0 {
		t.Fatal("world fixture not found")
	}
	monsters, context, ok := worldSourceMonsters(itemID, sourceLine, groupID, choicePosition, itemPosition, rt)
	if !ok || context == "" {
		t.Fatalf("valid source rejected: ok=%v context=%q", ok, context)
	}
	for i, monster := range monsters {
		if monster.MonsterID <= 0 || monster.Monster == "" || monster.Chance <= 0 {
			t.Fatalf("invalid monster row: %#v", monster)
		}
		if i > 0 {
			previous := monsters[i-1]
			if math.Abs(previous.Chance-monster.Chance) < 1e-12 && previous.Level > monster.Level {
				t.Fatalf("world candidates are not level ascending: %#v then %#v", previous, monster)
			}
			if math.Abs(previous.Chance-monster.Chance) < 1e-12 && previous.Level == monster.Level && strings.ToLower(previous.Monster) > strings.ToLower(monster.Monster) {
				t.Fatalf("world candidates are not alphabetical within level: %#v then %#v", previous, monster)
			}
		}
	}
	if _, _, ok := worldSourceMonsters(itemID, sourceLine, groupID, choicePosition+1, itemPosition, rt); ok {
		t.Fatal("wrong choicePosition unexpectedly matched world source")
	}
	if _, _, ok := worldSourceMonsters(itemID, sourceLine, groupID, choicePosition, itemPosition+99999, rt); ok {
		t.Fatal("wrong itemPosition unexpectedly matched world source")
	}
}

func TestWorldSourceEndpointRejectsMalformedAndDoesNotClaimMapMatch(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	bad := httptest.NewRecorder()
	handleWorldSourceMonsters(bad, httptest.NewRequest(http.MethodGet, "/api/world-source-monsters?itemId=-1", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad params status=%d want=400", bad.Code)
	}

	rt := activeRuntime("kiss")
	for _, rule := range rt.server.WorldRules {
		for choiceIndex, group := range rule.Groups {
			items := rt.resolved[group.GroupID]
			if len(items) == 0 {
				continue
			}
			itemID := items[0].ItemID
			drop := ItemDrop{ItemID: itemID, SourceLine: rule.SourceLine, GroupID: group.GroupID, ChoicePosition: choiceIndex + 1, ItemPosition: items[0].Position}
			url := "/api/world-source-monsters?server=kiss&itemId=" + itoa(drop.ItemID) + "&sourceLine=" + itoa(drop.SourceLine) + "&groupId=" + itoa(drop.GroupID) + "&choicePosition=" + itoa(drop.ChoicePosition) + "&itemPosition=" + itoa(drop.ItemPosition)
			recorder := httptest.NewRecorder()
			handleWorldSourceMonsters(recorder, httptest.NewRequest(http.MethodGet, url, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Monsters          []WorldSourceMonster `json:"monsters"`
				ContextMatchKnown bool                 `json:"contextMatchKnown"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.ContextMatchKnown {
				t.Fatal("world endpoint falsely claims confirmed monster-to-map context")
			}
			return
		}
	}
	t.Fatal("world source fixture not found")
}

func itoa(value int) string { return strconv.Itoa(value) }
