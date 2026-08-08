package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func worldSlotBySourceLine(t *testing.T, slots []DropSlot, sourceLine int) DropSlot {
	t.Helper()
	for _, slot := range slots {
		if slot.SourceLine == sourceLine {
			return slot
		}
	}
	t.Fatalf("world slot sourceLine=%d not found", sourceLine)
	return DropSlot{}
}

func dropItemByID(t *testing.T, slot DropSlot, groupID, itemID int) []DropItem {
	t.Helper()
	for _, choice := range slot.Choices {
		if choice.GroupID != groupID {
			continue
		}
		result := make([]DropItem, 0, 2)
		for _, item := range choice.Items {
			if item.ItemID == itemID {
				result = append(result, item)
			}
		}
		if len(result) != 0 {
			return result
		}
	}
	t.Fatalf("item %d in group %d not found", itemID, groupID)
	return nil
}

func TestHostileSpiritWorldDropsIncludeObservedSoulBeadsAndGoldenDesertWeaponChest(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	monster := store.monstersByID[85]
	if monster == nil || monster.Name != "Враждебный дух" || monster.Level != 50 {
		t.Fatalf("unexpected hostile spirit: %#v", monster)
	}
	if got := store.itemNames[835221]; got != "Четки души" {
		t.Fatalf("item 835221=%q want Четки души", got)
	}
	if got := store.itemNames[808100]; got != "Сундук с оружием из золотой пустыни" {
		t.Fatalf("item 808100=%q want golden desert weapon chest", got)
	}

	for _, server := range []string{"kiss", "original"} {
		rt := activeRuntime(server)
		slots := monsterWorldDropView(monster, rt)
		if got, want := len(slots), monsterWorldRuleCount(monster, rt); got != want || got == 0 {
			t.Fatalf("%s world slots=%d count=%d", server, got, want)
		}

		beadsSlot := worldSlotBySourceLine(t, slots, 57)
		beads := dropItemByID(t, beadsSlot, 44, 835221)
		if len(beads) != 3 {
			t.Fatalf("%s soul beads variants=%d want=3", server, len(beads))
		}
		wantChances := []float64{0.85, 0.10, 0.05}
		wantQty := []int{1, 3, 5}
		for i := range beads {
			if math.Abs(beads[i].BaseAttemptChance-wantChances[i]) > 1e-12 || beads[i].Quantity != wantQty[i] {
				t.Fatalf("%s soul beads variant %d=%#v", server, i, beads[i])
			}
		}

		chestSlot := worldSlotBySourceLine(t, slots, 144)
		chestRows := dropItemByID(t, chestSlot, 9999918, 808100)
		if len(chestRows) != 1 || math.Abs(chestRows[0].BaseAttemptChance-0.36) > 1e-12 || chestRows[0].Quantity != 1 {
			t.Fatalf("%s golden desert weapon chest=%#v", server, chestRows)
		}

		contents := chestContents(808100, rt)
		if contents == nil || len(contents.Items) != 40 {
			t.Fatalf("%s golden desert weapon chest contents=%#v", server, contents)
		}
	}
}

func TestHostileSpiritAppearsInWorldCandidateListsForObservedItems(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	rt := activeRuntime("kiss")
	tests := []struct {
		itemID         int
		sourceLine     int
		groupID        int
		choicePosition int
		itemPosition   int
	}{
		{835221, 57, 44, 1, 1},
		{808100, 144, 9999918, 1, 1},
	}
	for _, test := range tests {
		monsters, _, ok := worldSourceMonsters(test.itemID, test.sourceLine, test.groupID, test.choicePosition, test.itemPosition, rt)
		if !ok {
			t.Fatalf("item %d world source rejected", test.itemID)
		}
		found := false
		for _, monster := range monsters {
			if monster.MonsterID == 85 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Враждебный дух missing from candidates for item %d", test.itemID)
		}
	}
}

func TestMonsterWorldDropEndpointIsLazySafeAndDoesNotClaimLocationMatch(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	bad := httptest.NewRecorder()
	handleMonsterWorldDrops(bad, httptest.NewRequest(http.MethodGet, "/api/monster-world-drops?monsterId=-1", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad params status=%d want=400", bad.Code)
	}

	recorder := httptest.NewRecorder()
	handleMonsterWorldDrops(recorder, httptest.NewRequest(http.MethodGet, "/api/monster-world-drops?server=kiss&monsterId=85", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		MonsterID         int        `json:"monsterId"`
		Slots             []DropSlot `json:"slots"`
		ContextMatchKnown bool       `json:"contextMatchKnown"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.MonsterID != 85 || len(response.Slots) == 0 {
		t.Fatalf("unexpected response: monster=%d slots=%d", response.MonsterID, len(response.Slots))
	}
	if response.ContextMatchKnown {
		t.Fatal("monster world endpoint falsely claims confirmed location context")
	}
}

func TestEveryWorldRuleGroupResolvesForEveryServer(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for _, server := range []string{"kiss", "original"} {
		rt := activeRuntime(server)
		for _, rule := range rt.server.WorldRules {
			for _, group := range rule.Groups {
				items, ok := rt.resolved[group.GroupID]
				if !ok {
					t.Fatalf("%s world source line %d references missing group %d", server, rule.SourceLine, group.GroupID)
				}
				for _, item := range items {
					if store.itemsByID[item.ItemID] == nil {
						t.Fatalf("%s world source line %d group %d references missing item %d", server, rule.SourceLine, group.GroupID, item.ItemID)
					}
				}
			}
		}
	}
}
