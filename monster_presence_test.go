package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func requestJSON(t *testing.T, handler http.HandlerFunc, target string, body string) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(http.MethodGet, target, nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	var payload map[string]any
	if rec.Body.Len() > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s: %v\n%s", target, err, rec.Body.String())
		}
	}
	return rec.Code, payload
}

func TestMonsterPresenceCountsAndKnownServerDifferences(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if got := len(store.monsterPresence["original"]); got != 609 {
		t.Fatalf("original monster presence=%d want=609", got)
	}
	if got := len(store.monsterPresence["kiss"]); got != 677 {
		t.Fatalf("kiss monster presence=%d want=677", got)
	}
	cases := []struct {
		id             int
		original, kiss bool
	}{
		{id: 85, original: true, kiss: true},     // Враждебный дух
		{id: 11026, original: true, kiss: false}, // Хрусталиск
		{id: 1122, original: false, kiss: true},  // Вервиндль
		{id: 20026, original: false, kiss: true}, // Карад
		{id: 253, original: false, kiss: false},  // запись есть в общей таблице, но не размещена в переданном regen-наборе
	}
	for _, tc := range cases {
		_, original := store.monsterPresence["original"][tc.id]
		_, kiss := store.monsterPresence["kiss"][tc.id]
		if original != tc.original || kiss != tc.kiss {
			t.Fatalf("monster %d presence original=%v kiss=%v want original=%v kiss=%v", tc.id, original, kiss, tc.original, tc.kiss)
		}
	}
}

func TestMonsterCatalogAndDetailRespectSelectedServer(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	for server, want := range map[string]int{"original": 609, "kiss": 677} {
		code, payload := requestJSON(t, handleMonsters, "/api/monsters?server="+server+"&pageSize=8", "")
		if code != http.StatusOK {
			t.Fatalf("catalog %s status=%d", server, code)
		}
		if got := int(payload["total"].(float64)); got != want {
			t.Fatalf("catalog %s total=%d want=%d", server, got, want)
		}
	}
	for _, tc := range []struct {
		server string
		id     int
		want   int
	}{
		{"original", 11026, http.StatusOK},
		{"kiss", 11026, http.StatusNotFound},
		{"original", 1122, http.StatusNotFound},
		{"kiss", 1122, http.StatusOK},
		{"original", 253, http.StatusNotFound},
		{"kiss", 253, http.StatusNotFound},
	} {
		code, _ := requestJSON(t, handleMonster, "/api/monsters/"+strconvI(tc.id)+"?server="+tc.server, "")
		if code != tc.want {
			t.Fatalf("monster %d server=%s status=%d want=%d", tc.id, tc.server, code, tc.want)
		}
	}
}

func TestSearchAndFavoritesDoNotLeakMonstersAcrossServers(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	code, original := requestJSON(t, handleSearch, "/api/search?q=11026&server=original", "")
	if code != http.StatusOK || len(original["monsters"].([]any)) != 1 {
		t.Fatalf("original search did not return monster ID 11026: code=%d payload=%#v", code, original)
	}
	code, kiss := requestJSON(t, handleSearch, "/api/search?q=11026&server=kiss", "")
	if code != http.StatusOK || len(kiss["monsters"].([]any)) != 0 {
		t.Fatalf("kiss search leaked original-only monster ID 11026: code=%d payload=%#v", code, kiss)
	}

	body := `{"keys":["monster:11026","monster:1122"],"server":"original","page":1,"pageSize":24}`
	code, favorites := requestJSON(t, handleFavorites, "/api/favorites", body)
	if code != http.StatusOK {
		t.Fatalf("favorites status=%d payload=%#v", code, favorites)
	}
	rows := favorites["rows"].([]any)
	if len(rows) != 1 || int(rows[0].(map[string]any)["id"].(float64)) != 11026 {
		t.Fatalf("favorites leaked wrong-server monster: %#v", rows)
	}
}

func TestDropSourcesAndWorldCandidatesRespectMonsterPresence(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	// Карад (20026) has direct-drop rules in both embedded server tables, but the
	// supplied placement data confirms it only for Kiss. The item source list
	// therefore must not expose it for Original.
	const itemID = 1055094
	for server, wantKarat := range map[string]bool{"original": false, "kiss": true} {
		drops := itemDropSources(itemID, activeRuntime(server))
		found := false
		for _, drop := range drops {
			if drop.MonsterID == 20026 {
				found = true
			}
			if drop.MonsterID > 0 && !monsterVisible(activeRuntime(server), drop.MonsterID) {
				t.Fatalf("%s item sources contain hidden monster %d", server, drop.MonsterID)
			}
		}
		if found != wantKarat {
			t.Fatalf("%s Карад source=%v want=%v", server, found, wantKarat)
		}
	}

	// Every expanded world-drop candidate must belong to the selected server.
	rt := activeRuntime("kiss")
	for _, rule := range rt.server.WorldRules {
		if len(rule.Groups) == 0 {
			continue
		}
		group := rule.Groups[0]
		items := rt.resolved[group.GroupID]
		if len(items) == 0 {
			continue
		}
		monsters, _, ok := worldSourceMonsters(items[0].ItemID, rule.SourceLine, group.GroupID, 1, items[0].Position, rt)
		if !ok {
			continue
		}
		for _, monster := range monsters {
			if !monsterVisible(rt, monster.MonsterID) {
				t.Fatalf("world candidates contain hidden monster %d", monster.MonsterID)
			}
		}
		return
	}
	t.Fatal("no usable world rule fixture")
}

func strconvI(value int) string {
	if value == 0 {
		return "0"
	}
	buf := [24]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func TestRecentMonsterViewsRemainServerScoped(t *testing.T) {
	values := []recentViewEntry{
		{Type: "monster", ID: 85, Name: "Враждебный дух", Server: "kiss"},
		{Type: "monster", ID: 85, Name: "Враждебный дух", Server: "original"},
		{Type: "monster", ID: 85, Name: "bad", Server: "unknown"},
		{Type: "item", ID: 85, Name: "Предмет", Server: "kiss"},
	}
	got := sanitizeRecentViews(values, 8)
	if len(got) != 3 {
		t.Fatalf("recent views=%#v want 3 valid server-scoped entries", got)
	}
	if got[0].Server != "kiss" || got[1].Server != "original" {
		t.Fatalf("monster server scope was not preserved: %#v", got)
	}
	if got[2].Type != "item" || got[2].Server != "" {
		t.Fatalf("item view must not retain a server scope: %#v", got[2])
	}
}
