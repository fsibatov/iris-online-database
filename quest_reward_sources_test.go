package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestQuestRewardProjectionCountsAndKinds(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	total := 0
	titleRelations := 0
	recipeRelations := 0
	titleIndexes := make(map[int]struct{})
	recipeIDs := make(map[int]struct{})
	questIDs := make(map[int]struct{})
	for itemID, rewards := range store.questRewards {
		item := store.itemsByID[itemID]
		if item == nil {
			t.Fatalf("quest reward target item %d is missing", itemID)
		}
		_, isRecipe := store.itemRecipes[itemID]
		for _, reward := range rewards {
			total++
			questIDs[reward.QuestID] = struct{}{}
			if reward.RewardType != "default" {
				t.Fatalf("unexpected reward type in current resources: %+v", reward)
			}
			if isTitleItem(item) {
				titleRelations++
				titleIndexes[item.TitleIndex] = struct{}{}
			}
			if isRecipe {
				recipeRelations++
				recipeIDs[itemID] = struct{}{}
			}
			if !isTitleItem(item) && !isRecipe {
				t.Fatalf("unsupported quest reward target leaked: %+v", reward)
			}
		}
	}

	if total != 81 || len(store.questRewards) != 75 || len(questIDs) != 80 {
		t.Fatalf(
			"quest reward totals=%d items=%d quests=%d want=81/75/80",
			total,
			len(store.questRewards),
			len(questIDs),
		)
	}
	if titleRelations != 62 || len(titleIndexes) != 56 {
		t.Fatalf(
			"title quest rewards=%d unique titles=%d want=62/56",
			titleRelations,
			len(titleIndexes),
		)
	}
	if recipeRelations != 19 || len(recipeIDs) != 19 {
		t.Fatalf(
			"recipe quest rewards=%d unique recipes=%d want=19/19",
			recipeRelations,
			len(recipeIDs),
		)
	}
}

func TestQuestRewardFixturesAreExact(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}

	student := store.questRewards[807001]
	if len(student) != 3 {
		t.Fatalf("student title reward count=%d want=3", len(student))
	}
	gotIDs := make([]int, 0, len(student))
	for _, reward := range student {
		gotIDs = append(gotIDs, reward.QuestID)
		if reward.Quest != "Первый подвиг" || reward.Quantity != 1 {
			t.Fatalf("unexpected student reward: %+v", reward)
		}
	}
	sort.Ints(gotIDs)
	wantIDs := []int{20, 21, 22}
	for index := range wantIDs {
		if gotIDs[index] != wantIDs[index] {
			t.Fatalf("student quest IDs=%v want=%v", gotIDs, wantIDs)
		}
	}

	recipe := store.questRewards[891002]
	if len(recipe) != 1 || recipe[0].QuestID != 78 ||
		recipe[0].Quest != "На что годятся пауки - 1" {
		t.Fatalf("spider sausage recipe reward mismatch: %+v", recipe)
	}

	for _, rewards := range store.questRewards {
		for _, reward := range rewards {
			if reward.QuestID == 4033 {
				t.Fatalf("unconfirmed quest 4033 reward leaked: %+v", reward)
			}
		}
	}
}

func TestTitleQuestRewardsAreCommonSourcesWithoutServerSubstitution(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	title := store.titlesByIndex[1]
	if title == nil {
		t.Fatal("title 1 is missing")
	}

	for _, server := range []string{"original", "kiss"} {
		drops := titleDropSources(title, activeRuntime(server))
		questIDs := make([]int, 0, 3)
		for _, drop := range drops {
			if drop.Source != "Награда за задание" {
				continue
			}
			questIDs = append(questIDs, drop.QuestID)
			if drop.Context != "Гарантированная награда" || drop.RewardType != "default" {
				t.Fatalf("server=%s reward presentation mismatch: %+v", server, drop)
			}
		}
		sort.Ints(questIDs)
		if len(questIDs) != 3 || questIDs[0] != 20 || questIDs[1] != 21 || questIDs[2] != 22 {
			t.Fatalf("server=%s quest IDs=%v want=[20 21 22]", server, questIDs)
		}
		if !titleHasKnownSource(title, activeRuntime(server)) {
			t.Fatalf("server=%s title quest source is not indexed as known", server)
		}
	}
}

func TestRecipeQuestRewardIsExposedAsConfirmedSource(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	item := store.itemsByID[891002]
	if item == nil {
		t.Fatal("recipe 891002 is missing")
	}
	for _, server := range []string{"original", "kiss"} {
		summary := recipeSummary(item, activeRuntime(server))
		preview, ok := summary["sourcePreview"].(map[string]any)
		if !ok {
			t.Fatalf("server=%s source preview is missing: %+v", server, summary)
		}
		if preview["type"] != "Задание" || preview["name"] != "На что годятся пауки - 1" {
			t.Fatalf("server=%s source preview=%+v", server, preview)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/items/891002?server=kiss", nil)
	rec := httptest.NewRecorder()
	handleItem(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Drops []ItemDrop `json:"drops"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, drop := range payload.Drops {
		if drop.Source == "Награда за задание" && drop.QuestID == 78 {
			found = true
			if drop.Context != "Гарантированная награда" || drop.Quantity != 1 {
				t.Fatalf("quest reward payload mismatch: %+v", drop)
			}
		}
	}
	if !found {
		t.Fatal("quest completion reward is missing from recipe detail")
	}
}
