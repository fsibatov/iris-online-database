package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type TransformationCharacteristic struct {
	Name     string   `json:"name"`
	Value    *float64 `json:"value,omitempty"`
	Percent  bool     `json:"percent,omitempty"`
	Positive bool     `json:"positive"`
	Text     string   `json:"text"`
}

type TransformationSkillStatus struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type TransformationSkill struct {
	Position          int                            `json:"position"`
	Name              string                         `json:"name"`
	SkillTooltipIndex int                            `json:"skillTooltipIndex,omitempty"`
	EffectID          int                            `json:"effectId,omitempty"`
	CooldownMs        int                            `json:"cooldownMs,omitempty"`
	ManaCost          int                            `json:"manaCost,omitempty"`
	DurationMs        int                            `json:"durationMs,omitempty"`
	SkillType         int                            `json:"skillType,omitempty"`
	ApplyType         int                            `json:"applyType,omitempty"`
	Target            string                         `json:"target,omitempty"`
	EffectText        string                         `json:"effectText,omitempty"`
	Characteristics   []TransformationCharacteristic `json:"characteristics,omitempty"`
	IsSelfBuff        bool                           `json:"isSelfBuff"`
	IsFriendlyBuff    bool                           `json:"isFriendlyBuff"`
}

type TransformationCard struct {
	ItemID              int                            `json:"itemId"`
	Name                string                         `json:"name"`
	Quality             string                         `json:"quality,omitempty"`
	QualityID           int                            `json:"qualityId,omitempty"`
	MonsterID           int                            `json:"monsterId"`
	FormName            string                         `json:"formName,omitempty"`
	DurationMs          int                            `json:"durationMs,omitempty"`
	RunSpeed            float64                        `json:"runSpeed"`
	EffectiveRunSpeed   float64                        `json:"effectiveRunSpeed"`
	SpeedDelta          float64                        `json:"speedDelta"`
	SpeedDeltaPercent   float64                        `json:"speedDeltaPercent"`
	FormEffectText      string                         `json:"formEffectText,omitempty"`
	FormCharacteristics []TransformationCharacteristic `json:"formCharacteristics,omitempty"`
	Buffs               []string                       `json:"buffs,omitempty"`
	Skills              []TransformationSkill          `json:"skills,omitempty"`
}

type transformationSupplement struct {
	SchemaVersion      int                  `json:"schemaVersion"`
	BasePlayerRunSpeed float64              `json:"basePlayerRunSpeed"`
	Cards              []TransformationCard `json:"cards"`
}

var transformationCards []TransformationCard
var transformationByItem map[int]*TransformationCard
var transformationSearch []searchDocument
var transformationBuffNames []string
var basePlayerRunSpeed float64

var blockedTransformationCharacteristicNames = map[string]bool{
	"npc] превращение":              true,
	"общий / подтип] трансформация": true,
	"во время превращения сильно растет [восполнение природного здоровье], атака и перемещение невозможны": true,
	"маг.] число основной атаки": true,
	"физ.] число основной атаки": true,
	"превращение критика в текущего принца. выглядит слабым и если ударить может заплакать": true,
	"от максимума":                   true,
	"пока ваши ом не достигнут":      true,
	"расходует ману вместо здоровья": true,
	"увеличение":                     true,
}

func normalizeTransformationCharacteristicName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(trimmed, "ё", "е"), " ", " "))
	lower = strings.Join(strings.Fields(lower), " ")
	if blockedTransformationCharacteristicNames[lower] {
		return ""
	}
	switch {
	case strings.HasPrefix(lower, "вас лечат"):
		return "Получаемое исцеление"
	case strings.HasPrefix(lower, "лечение [здоровье]"):
		return "Восстановление здоровья"
	case strings.HasPrefix(lower, "получению агрессии") || strings.HasPrefix(lower, "получаемой агрессии"):
		return "Получение агрессии"
	case strings.HasPrefix(lower, "маг. стойкости"):
		return "Магическая стойкость"
	case strings.HasPrefix(lower, "физ. стойкости"):
		return "Физическая стойкость"
	case strings.HasPrefix(lower, "ко всему урону"):
		return "Общий урон"
	case strings.HasPrefix(lower, "защищает от любых атак"):
		return "Защита от любых атак"
	case strings.HasPrefix(lower, "насыщение маг. разрушений"):
		return "Магическое поглощение"
	case lower == "иммунитет к яду" || lower == "иммунитет к ядам":
		return "Иммунитет к ядам"
	case strings.HasPrefix(lower, "иммунитет к обездвижению") || strings.HasPrefix(lower, "иммунитет к обездвиживанию"):
		return "Иммунитет к обездвиживанию"
	case strings.HasPrefix(lower, "снятие негативных эффектов") || strings.HasPrefix(lower, "снятие отрицательных эффектов"):
		return "Снятие отрицательных эффектов"
	case lower == "меткости":
		return "Меткость"
	case strings.HasPrefix(lower, "может разрушить защиту ивентового босса"):
		return "Разрушение защиты ивентового босса"
	default:
		return trimmed
	}
}

func transformationCharacteristicFilterKey(row TransformationCharacteristic) string {
	name := normalizeTransformationCharacteristicName(row.Name)
	if name == "" {
		return ""
	}
	return characteristicFilterKey(name, row.Percent)
}

func loadTransformationCards() error {
	raw, err := embedded.ReadFile("assets/transformation_cards.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать карты превращения: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть карты превращения: %w", err)
	}
	defer gz.Close()
	var supplement transformationSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать карты превращения: %w", err)
	}
	if supplement.SchemaVersion != 1 || supplement.BasePlayerRunSpeed <= 0 || len(supplement.Cards) == 0 {
		return fmt.Errorf("некорректный пакет карт превращения")
	}
	transformationCards = append([]TransformationCard(nil), supplement.Cards...)
	basePlayerRunSpeed = supplement.BasePlayerRunSpeed
	transformationByItem = make(map[int]*TransformationCard, len(transformationCards))
	transformationSearch = make([]searchDocument, len(transformationCards))
	buffs := map[string]bool{}
	for i := range transformationCards {
		card := &transformationCards[i]
		if card.ItemID <= 0 || strings.TrimSpace(card.Name) == "" || card.MonsterID <= 0 {
			return fmt.Errorf("некорректная карта превращения: %+v", card)
		}
		if _, duplicate := transformationByItem[card.ItemID]; duplicate {
			return fmt.Errorf("повторяющаяся карта превращения: %d", card.ItemID)
		}
		transformationByItem[card.ItemID] = card
		parts := []string{strconv.Itoa(card.ItemID), card.Name, card.FormName}
		parts = append(parts, card.Buffs...)
		for _, skill := range card.Skills {
			parts = append(parts, skill.Name, skill.EffectText)
		}
		transformationSearch[i] = newSearchDocument(strings.Join(parts, " "))
		collect := func(rows []TransformationCharacteristic) {
			for _, row := range rows {
				if !row.Positive {
					continue
				}
				key := transformationCharacteristicFilterKey(row)
				if key != "" {
					buffs[key] = true
				}
			}
		}
		collect(card.FormCharacteristics)
		for _, skill := range card.Skills {
			if skill.IsFriendlyBuff || skill.IsSelfBuff {
				collect(skill.Characteristics)
			}
		}
	}
	transformationBuffNames = sortedKeys(buffs)
	return nil
}

func isTransformationItem(itemID int) bool {
	return transformationByItem[itemID] != nil
}

func transformationCharacteristicMatch(card *TransformationCard, name string) (float64, bool, bool) {
	if card == nil || strings.TrimSpace(name) == "" {
		return 0, false, false
	}
	found := false
	hasNumericValue := false
	best := 0.0
	consider := func(rows []TransformationCharacteristic) {
		for _, row := range rows {
			if !row.Positive || transformationCharacteristicFilterKey(row) != name {
				continue
			}
			found = true
			if row.Value == nil {
				continue
			}
			if !hasNumericValue || *row.Value > best {
				best = *row.Value
				hasNumericValue = true
			}
		}
	}
	consider(card.FormCharacteristics)
	for _, skill := range card.Skills {
		if skill.IsFriendlyBuff || skill.IsSelfBuff {
			consider(skill.Characteristics)
		}
	}
	return best, found, hasNumericValue
}

func transformationCharacteristicValue(card *TransformationCard, name string) (float64, bool) {
	value, found, _ := transformationCharacteristicMatch(card, name)
	return value, found
}

func transformationHasAllySkill(card *TransformationCard) bool {
	if card == nil {
		return false
	}
	for _, skill := range card.Skills {
		if skill.ApplyType == 3 {
			return true
		}
	}
	return false
}

var blockedTransformationStatusNames = map[string]bool{
	"разъяренное чудище набрасывается": true,
	"на применившего":                  true,
	"после использования":              true,
	"каждые сек. здоровье":             true,
	"невозм. исп. умения":              true,
	"каждые сек. оз":                   true,
	"оз через секунд":                  true,
	"урону":                            true,
	"снятие по эффекту усиления и чар": true,
}

func transformationStatusName(row TransformationCharacteristic) string {
	name := normalizeTransformationCharacteristicName(row.Name)
	if name == "" {
		return ""
	}
	lower := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(name, "ё", "е")), " "))
	if blockedTransformationStatusNames[lower] {
		return ""
	}
	switch lower {
	case "оз каждую секунду", "оз каждые секунды":
		return "Потеря здоровья"
	case "ом каждую секунду", "ом каждые секунды":
		return "Потеря маны"
	case "оз и к ом каждую секунду", "оз и к ом каждые секунды":
		return "Потеря здоровья и маны"
	}
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return ""
		}
	}
	return name
}

func transformationSkillStatuses(card *TransformationCard) []TransformationSkillStatus {
	if card == nil {
		return nil
	}
	statuses := make([]TransformationSkillStatus, 0)
	seen := make(map[string]struct{})
	appendStatus := func(kind, name string) bool {
		name = strings.TrimSpace(name)
		if kind == "" || name == "" {
			return false
		}
		key := kind + "\x00" + strings.ToLower(name)
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
		statuses = append(statuses, TransformationSkillStatus{Kind: kind, Name: name})
		return true
	}
	for _, skill := range card.Skills {
		if skill.ApplyType != 1 && skill.ApplyType != 2 && skill.ApplyType != 3 {
			continue
		}
		represented := false
		for _, row := range skill.Characteristics {
			name := transformationStatusName(row)
			if name == "" {
				continue
			}
			represented = true
			kind := "effect"
			if skill.ApplyType == 2 || !row.Positive {
				kind = "debuff"
			} else if skill.DurationMs > 0 {
				kind = "buff"
			}
			appendStatus(kind, name)
		}
		if !represented && skill.ApplyType != 2 && skill.SkillType == 3 {
			appendStatus("effect", skill.Name)
		}
	}
	return statuses
}

func transformationSummary(card *TransformationCard, characteristic string) map[string]any {
	result := map[string]any{
		"id": card.ItemID, "itemId": card.ItemID, "name": card.Name, "quality": card.Quality,
		"qualityId": card.QualityID, "formName": card.FormName, "durationMs": card.DurationMs,
		"effectiveRunSpeed": card.EffectiveRunSpeed, "speedDelta": card.SpeedDelta,
		"speedDeltaPercent": card.SpeedDeltaPercent, "buffs": card.Buffs,
		"skillStatuses": transformationSkillStatuses(card),
	}
	if value, found, hasNumericValue := transformationCharacteristicMatch(card, characteristic); found {
		result["characteristicMatched"] = true
		if hasNumericValue {
			result["characteristicValue"] = value
		}
	}
	return result
}

func handleTransformations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/api/transformations" {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	qv := r.URL.Query()
	query, queryOK := limitedQueryValue(qv, "q", 160)
	characteristic, characteristicOK := limitedQueryValue(qv, "characteristic", 120)
	quality, qualityOK := limitedQueryValue(qv, "quality", 80)
	ally := qv.Get("ally")
	if !queryOK || !characteristicOK || !qualityOK {
		http.Error(w, "Слишком длинное значение фильтра.\n", http.StatusBadRequest)
		return
	}
	if ally != "" && ally != "1" {
		http.Error(w, "Некорректное значение фильтра.\n", http.StatusBadRequest)
		return
	}
	if characteristic != "" {
		valid := false
		for _, candidate := range transformationBuffNames {
			if candidate == characteristic {
				valid = true
				break
			}
		}
		if !valid {
			http.Error(w, "Неизвестный эффект.\n", http.StatusBadRequest)
			return
		}
	}
	sortMode, sortOK := limitedQueryValue(qv, "sort", 24)
	if !sortOK || (sortMode != "" && sortMode != "name" && sortMode != "speed" && sortMode != "duration" && sortMode != "characteristic") {
		http.Error(w, "Некорректный режим сортировки.\n", http.StatusBadRequest)
		return
	}
	order, orderOK := querySortOrder(qv)
	if !orderOK {
		http.Error(w, "Некорректный порядок сортировки.\n", http.StatusBadRequest)
		return
	}
	descending := order == "desc"
	if sortMode == "" {
		sortMode = "name"
	}
	page := clampInt(parseInt(qv, "page", 1), 1, 100000)
	pageSize := clampInt(parseInt(qv, "pageSize", 20), 8, 48)
	qualities := make(map[string]int)
	filtered := make([]*TransformationCard, 0, len(transformationCards))
	for i := range transformationCards {
		card := &transformationCards[i]
		if qualityName := strings.TrimSpace(card.Quality); qualityName != "" {
			if qualityID, ok := qualities[qualityName]; !ok || card.QualityID < qualityID {
				qualities[qualityName] = card.QualityID
			}
		}
		if query != "" && !matchesSearch(transformationSearch[i], query) {
			continue
		}
		if characteristic != "" {
			if _, ok := transformationCharacteristicValue(card, characteristic); !ok {
				continue
			}
		}
		if ally == "1" && !transformationHasAllySkill(card) {
			continue
		}
		if quality != "" && card.Quality != quality {
			continue
		}
		filtered = append(filtered, card)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := filtered[i], filtered[j]
		switch sortMode {
		case "name":
			if less, different := orderedCatalogNameLess(left.Name, right.Name, descending); different {
				return less
			}
		case "speed":
			if less, different := orderedFloatLess(left.EffectiveRunSpeed, right.EffectiveRunSpeed, descending); different {
				return less
			}
		case "duration":
			if less, different := orderedIntLess(left.DurationMs, right.DurationMs, descending); different {
				return less
			}
		case "characteristic":
			lv, lok, lNumeric := transformationCharacteristicMatch(left, characteristic)
			rv, rok, rNumeric := transformationCharacteristicMatch(right, characteristic)
			if lok != rok {
				return lok
			}
			if lNumeric != rNumeric {
				return lNumeric
			}
			if lNumeric {
				if less, different := orderedFloatLess(lv, rv, descending); different {
					return less
				}
			}
		}
		if left.Name != right.Name {
			return catalogNameLess(left.Name, right.Name)
		}
		return left.ItemID < right.ItemID
	})
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := min(total, start+pageSize)
	rows := make([]map[string]any, 0, end-start)
	for _, card := range filtered[start:end] {
		rows = append(rows, transformationSummary(card, characteristic))
	}
	writeJSON(w, map[string]any{
		"transformations": rows, "total": total, "page": page, "pageSize": pageSize,
		"pages":              max(1, (total+pageSize-1)/pageSize),
		"filters":            map[string]any{"characteristics": transformationBuffNames, "qualities": sortedQualityKeys(qualities)},
		"basePlayerRunSpeed": basePlayerRunSpeed,
	})
}

func handleTransformation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/transformations/"))
	if err != nil || id <= 0 {
		http.Error(w, "Некорректный идентификатор карты превращения.\n", http.StatusBadRequest)
		return
	}
	card := transformationByItem[id]
	if card == nil {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{
		"card": card, "basePlayerRunSpeed": basePlayerRunSpeed,
		"drops": itemDropSources(id, activeRuntime(r.URL.Query().Get("server"))),
	})
}
