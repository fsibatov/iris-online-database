package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestItemEnhancementUsesSourceProfileFromZeroThroughTen(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	item := store.itemsByID[1]
	if item == nil {
		t.Fatal("fixture item 1 is missing")
	}
	enhancement := itemEnhancement(item)
	if enhancement == nil {
		t.Fatal("enhancement for starter sword is missing")
	}
	if enhancement.ProfileID != 1 || enhancement.MaxLevel != 10 || len(enhancement.Levels) != 11 {
		t.Fatalf("unexpected enhancement shape: %#v", enhancement)
	}
	if enhancement.Levels[0].Label != "Без усиления" || enhancement.Levels[10].Label != "+10" {
		t.Fatalf("unexpected level labels: %q .. %q", enhancement.Levels[0].Label, enhancement.Levels[10].Label)
	}
	if len(enhancement.Levels[10].Stats) != 1 {
		t.Fatalf("+10 stats=%d want=1", len(enhancement.Levels[10].Stats))
	}
	stat := enhancement.Levels[10].Stats[0]
	if stat.Type != enhancePhysicalAttack || stat.BaseMin != 61 || stat.BaseMax != 68 || !stat.IsRange {
		t.Fatalf("unexpected +10 stat: %#v", stat)
	}
	if math.Abs(stat.Percent-132) > 1e-9 || stat.Bonus != 89 {
		t.Fatalf("+10 percent/bonus=%v/%d want=132/89", stat.Percent, stat.Bonus)
	}
}

func TestTransformationPolenyaPreservesConfirmedSpeedAndBuff(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	card := transformationByItem[1021002]
	if card == nil {
		t.Fatal("Polenya transformation card 1021002 is missing")
	}
	if card.MonsterID != 2 || card.RunSpeed != 320 || card.EffectiveRunSpeed != 320 {
		t.Fatalf("unexpected Polenya form: %#v", card)
	}
	if math.Abs(card.SpeedDeltaPercent-(-28.9)) > 1e-9 {
		t.Fatalf("speed delta percent=%v", card.SpeedDeltaPercent)
	}
	foundEvasion := map[string]bool{}
	for _, row := range card.FormCharacteristics {
		if row.Positive && row.Value != nil && *row.Value == 100 {
			foundEvasion[row.Name] = true
		}
	}
	if !foundEvasion["Физическое уклонение"] || !foundEvasion["Магическое уклонение"] {
		t.Fatalf("Polenya form evasion buffs missing: %#v", card.FormCharacteristics)
	}
	foundHooves := false
	for _, skill := range card.Skills {
		if skill.Name != "Усиление копыт" {
			continue
		}
		foundHooves = true
		if !skill.IsSelfBuff || skill.CooldownMs != 60000 || skill.DurationMs != 30000 {
			t.Fatalf("unexpected hoof skill metadata: %#v", skill)
		}
		foundSpeed := false
		for _, row := range skill.Characteristics {
			if row.Name == "Скорость бега" && row.Positive && !row.Percent && row.Value != nil && *row.Value == 300 {
				foundSpeed = true
			}
		}
		if !foundSpeed {
			t.Fatalf("+300 run-speed buff missing: %#v", skill.Characteristics)
		}
	}
	if !foundHooves {
		t.Fatal("Усиление копыт skill is missing")
	}
}

func TestTransformationAndTitleCharacteristicFiltersKeepUnitsSeparate(t *testing.T) {
	if characteristicFilterKey("Скорость бега", false) == characteristicFilterKey("Скорость бега", true) {
		t.Fatal("flat and percent characteristics must have different filter keys")
	}
	if characteristicFilterKey("Скорость бега", true) != "Скорость бега (%)" {
		t.Fatalf("unexpected percent key: %q", characteristicFilterKey("Скорость бега", true))
	}
}

func TestTransformationCatalogSortOrderSupportsDescendingAndAscending(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	getIDs := func(query string) []int {
		req := httptest.NewRequest(http.MethodGet, query, nil)
		rec := httptest.NewRecorder()
		handleTransformations(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("query=%s status=%d body=%s", query, rec.Code, rec.Body.String())
		}
		var payload struct {
			Transformations []struct {
				ID int `json:"id"`
			} `json:"transformations"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		ids := make([]int, 0, len(payload.Transformations))
		for _, row := range payload.Transformations {
			ids = append(ids, row.ID)
		}
		return ids
	}
	asc := getIDs("/api/transformations?sort=speed&order=asc&page=1&pageSize=24")
	desc := getIDs("/api/transformations?sort=speed&order=desc&page=1&pageSize=24")
	if len(asc) == 0 || len(desc) == 0 {
		t.Fatal("transformation catalog sort order returned no rows")
	}
	if asc[0] == desc[0] {
		t.Fatalf("ascending and descending transformation order look identical: asc=%v desc=%v", asc[:min(5, len(asc))], desc[:min(5, len(desc))])
	}
}

func TestTransformationFilterCharacteristicsAreCleanAndReadable(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/transformations?page=1&pageSize=8", nil)
	rec := httptest.NewRecorder()
	handleTransformations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Filters struct {
			Characteristics []string `json:"characteristics"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Filters.Characteristics) == 0 {
		t.Fatal("transformation characteristics are empty")
	}
	blocked := map[string]bool{
		"NPC] превращение": true,
		"Во время превращения сильно растет [восполнение природного Здоровье], атака и перемещение невозможны": true,
		"Общий / подтип] трансформация": true,
		"Маг.] число основной атаки":    true,
		"Физ.] число основной атаки":    true,
		"Превращение Критика в текущего принца. Выглядит слабым и если ударить может заплакать": true,
		"От максимума":                   true,
		"Пока ваши ОМ не достигнут (%)":  true,
		"Расходует ману вместо здоровья": true,
	}
	seen := map[string]bool{}
	for _, name := range payload.Filters.Characteristics {
		seen[name] = true
		if blocked[name] {
			t.Fatalf("blocked filter leaked into transformation list: %q", name)
		}
		if strings.Contains(name, "[") || strings.Contains(name, "]") {
			t.Fatalf("transformation filter must not contain broken brackets: %q", name)
		}
	}
	for _, name := range []string{"Получаемое исцеление", "Получение агрессии (%)", "Восстановление здоровья (%)", "Иммунитет к ядам"} {
		if !seen[name] {
			t.Fatalf("expected readable filter is missing: %q", name)
		}
	}
	if seen["Иммунитет к яду"] {
		t.Fatal("singular poison-immunity alias must be merged into \"Иммунитет к ядам\"")
	}
}

func TestTransformationBooleanCharacteristicDoesNotBecomePlusZero(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	name := "Иммунитет к ослаблению рукопашного урона"
	var card *TransformationCard
	for i := range transformationCards {
		candidate := &transformationCards[i]
		if _, found, numeric := transformationCharacteristicMatch(candidate, name); found {
			if numeric {
				t.Fatalf("boolean immunity unexpectedly has a numeric value: %#v", candidate)
			}
			card = candidate
			break
		}
	}
	if card == nil {
		t.Fatalf("fixture characteristic %q not found", name)
	}
	summary := transformationSummary(card, name)
	if summary["characteristicMatched"] != true {
		t.Fatalf("selected boolean characteristic is not marked as matched: %#v", summary)
	}
	if _, exists := summary["characteristicValue"]; exists {
		t.Fatalf("boolean characteristic must not expose a synthetic +0 value: %#v", summary)
	}
}

func TestTransformationCatalogSupportsRarityFilter(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/transformations?quality=Редкое&page=1&pageSize=48", nil)
	rec := httptest.NewRecorder()
	handleTransformations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Transformations []struct {
			Quality string `json:"quality"`
		} `json:"transformations"`
		Filters struct {
			Qualities []string `json:"qualities"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Transformations) == 0 {
		t.Fatal("rarity-filtered transformation catalog returned no rows")
	}
	for _, row := range payload.Transformations {
		if row.Quality != "Редкое" {
			t.Fatalf("unexpected quality in filtered transformations: %q", row.Quality)
		}
	}
	want := map[string]bool{"Обычное": false, "Необычное": false, "Редкое": false, "PvP": false}
	for _, quality := range payload.Filters.Qualities {
		if _, ok := want[quality]; ok {
			want[quality] = true
		}
	}
	for quality, found := range want {
		if !found {
			t.Fatalf("transformation rarity option is missing: %q", quality)
		}
	}
}

func TestTransformationSkillStatusesAreDeduplicatedAndTargetAware(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	card := transformationByItem[1026010]
	if card == nil {
		t.Fatal("known transformation card 1026010 is missing")
	}
	statuses := transformationSkillStatuses(card)
	if len(statuses) == 0 {
		t.Fatal("skill statuses are missing")
	}
	seen := map[string]bool{}
	want := map[string]bool{
		"debuff:Физическое уклонение": false,
		"debuff:Скорость бега":        false,
		"debuff:Магическое уклонение": false,
	}
	for _, status := range statuses {
		key := status.Kind + ":" + status.Name
		if seen[key] {
			t.Fatalf("duplicate skill status: %s", key)
		}
		seen[key] = true
		if _, ok := want[key]; ok {
			want[key] = true
		}
		if strings.ContainsAny(status.Name, "[]") {
			t.Fatalf("technical status leaked into summary: %q", status.Name)
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("expected status is missing: %s; got=%#v", key, statuses)
		}
	}

	mushroom := transformationByItem[1070014]
	if mushroom == nil {
		t.Fatal("known ally transformation card 1070014 is missing")
	}
	foundBuff := false
	for _, status := range transformationSkillStatuses(mushroom) {
		if status.Kind == "buff" && status.Name == "Размер" {
			foundBuff = true
		}
	}
	if !foundBuff {
		t.Fatal("ally-target size buff is missing from skill statuses")
	}

	tradeoff := transformationByItem[1021053]
	if tradeoff == nil {
		t.Fatal("known self-buff tradeoff card 1021053 is missing")
	}
	hasSpeedBuff := false
	hasDefenseDebuff := false
	for _, status := range transformationSkillStatuses(tradeoff) {
		if status.Kind == "buff" && status.Name == "Скорость бега" {
			hasSpeedBuff = true
		}
		if status.Kind == "debuff" && status.Name == "Все виды защиты" {
			hasDefenseDebuff = true
		}
	}
	if !hasSpeedBuff || !hasDefenseDebuff {
		t.Fatalf("mixed self effect is classified incorrectly: %#v", transformationSkillStatuses(tradeoff))
	}

	commander := transformationByItem[1026016]
	if commander == nil {
		t.Fatal("known periodic-damage card 1026016 is missing")
	}
	healthLoss := 0
	for _, status := range transformationSkillStatuses(commander) {
		if status.Kind == "debuff" && status.Name == "Потеря здоровья" {
			healthLoss++
		}
		if status.Name == "ОЗ каждую секунду" || status.Name == "ОЗ каждые секунды" {
			t.Fatalf("grammatical duplicate leaked into status summary: %#v", transformationSkillStatuses(commander))
		}
	}
	if healthLoss != 1 {
		t.Fatalf("periodic health-loss status must be deduplicated: %#v", transformationSkillStatuses(commander))
	}

	niil := transformationByItem[1021022]
	if niil == nil {
		t.Fatal("Niil transformation card 1021022 is missing")
	}
	wantNiil := map[string]bool{
		"effect:Волна исцеления":               false,
		"effect:Снятие отрицательных эффектов": false,
	}
	for _, status := range transformationSkillStatuses(niil) {
		key := status.Kind + ":" + status.Name
		if _, ok := wantNiil[key]; ok {
			wantNiil[key] = true
		}
	}
	for key, found := range wantNiil {
		if !found {
			t.Fatalf("Niil utility status is missing: %s; got=%#v", key, transformationSkillStatuses(niil))
		}
	}
}

func TestTransformationCatalogFiltersSkillsTargetingAllies(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/transformations?ally=1&page=1&pageSize=48", nil)
	rec := httptest.NewRecorder()
	handleTransformations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Transformations []struct {
			ID int `json:"id"`
		} `json:"transformations"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total == 0 || len(payload.Transformations) == 0 {
		t.Fatal("ally-target transformation filter returned no rows")
	}
	foundMushroom := false
	for _, row := range payload.Transformations {
		card := transformationByItem[row.ID]
		if card == nil || !transformationHasAllySkill(card) {
			t.Fatalf("ally filter returned card without ally-target skill: %d", row.ID)
		}
		if row.ID == 1070014 {
			foundMushroom = true
		}
	}
	if !foundMushroom {
		t.Fatal("known ally-target card 1070014 is missing from ally filter")
	}

	badReq := httptest.NewRequest(http.MethodGet, "/api/transformations?ally=yes", nil)
	badRec := httptest.NewRecorder()
	handleTransformations(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid ally filter status=%d want=%d", badRec.Code, http.StatusBadRequest)
	}
}
