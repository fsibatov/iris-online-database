package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

const enhancementMaxLevel = 10

const (
	enhancePhysicalAttack  = 1201
	enhanceMagicAttack     = 1202
	enhancePhysicalDefense = 1203
	enhanceMagicDefense    = 1204
	enhanceHealing         = 1220
)

type enhancementSourceRow struct {
	Equip  int       `json:"equip"`
	Type   int       `json:"type"`
	Values []float64 `json:"values"`
}

type enhancementSupplement struct {
	SchemaVersion int                               `json:"schemaVersion"`
	Levels        int                               `json:"levels"`
	Profiles      map[string][]enhancementSourceRow `json:"profiles"`
}

type ItemEnhancementStat struct {
	Type    int     `json:"type"`
	Name    string  `json:"name"`
	BaseMin int     `json:"baseMin,omitempty"`
	BaseMax int     `json:"baseMax,omitempty"`
	Base    int     `json:"base,omitempty"`
	Bonus   int     `json:"bonus"`
	Percent float64 `json:"percent"`
	IsRange bool    `json:"isRange,omitempty"`
}

type ItemEnhancementLevel struct {
	Level int                   `json:"level"`
	Label string                `json:"label"`
	Stats []ItemEnhancementStat `json:"stats"`
}

type ItemEnhancement struct {
	ProfileID int                    `json:"profileId"`
	MaxLevel  int                    `json:"maxLevel"`
	Levels    []ItemEnhancementLevel `json:"levels"`
}

var enhancementProfiles map[int][]enhancementSourceRow

func loadEnhancementProfiles() error {
	raw, err := embedded.ReadFile("assets/item_enhancements.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать уровни усиления предметов: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть уровни усиления предметов: %w", err)
	}
	defer gz.Close()
	var supplement enhancementSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать уровни усиления предметов: %w", err)
	}
	if supplement.SchemaVersion != 1 || supplement.Levels != enhancementMaxLevel {
		return fmt.Errorf("неподдерживаемые данные усиления: schema=%d levels=%d", supplement.SchemaVersion, supplement.Levels)
	}
	enhancementProfiles = make(map[int][]enhancementSourceRow, len(supplement.Profiles))
	for key, rows := range supplement.Profiles {
		profileID, err := strconv.Atoi(key)
		if err != nil || profileID <= 0 {
			return fmt.Errorf("некорректный индекс усиления: %q", key)
		}
		for _, row := range rows {
			if len(row.Values) != enhancementMaxLevel {
				return fmt.Errorf("профиль усиления %d содержит %d уровней", profileID, len(row.Values))
			}
		}
		enhancementProfiles[profileID] = append([]enhancementSourceRow(nil), rows...)
	}
	return nil
}

func enhancementBase(item *Item, optionType int) (name string, baseMin, baseMax, base int, isRange bool) {
	if item == nil {
		return "", 0, 0, 0, false
	}
	switch optionType {
	case enhancePhysicalAttack:
		if item.PhysicalMax <= 0 {
			return "", 0, 0, 0, false
		}
		return "Физическая атака", item.PhysicalMin, item.PhysicalMax, item.PhysicalMax, true
	case enhanceMagicAttack:
		if item.MagicMax <= 0 {
			return "", 0, 0, 0, false
		}
		return "Магическая атака", item.MagicMin, item.MagicMax, item.MagicMax, true
	case enhancePhysicalDefense:
		if item.PhysicalDefense <= 0 {
			return "", 0, 0, 0, false
		}
		return "Физическая защита", 0, 0, item.PhysicalDefense, false
	case enhanceMagicDefense:
		if item.MagicDefense <= 0 {
			return "", 0, 0, 0, false
		}
		return "Магическая защита", 0, 0, item.MagicDefense, false
	case enhanceHealing:
		if item.Heal <= 0 {
			return "", 0, 0, 0, false
		}
		return "Лечение", 0, 0, item.Heal, false
	default:
		return "", 0, 0, 0, false
	}
}

func enhancementBonus(base int, percent float64) int {
	if base <= 0 || percent == 0 {
		return 0
	}
	// ItemTipWindow_Set.cpp truncates the positive result when assigning it to a short.
	return int(math.Trunc(float64(base) * percent / 100.0))
}

func itemEnhancement(item *Item) *ItemEnhancement {
	if item == nil || item.EnhancedIndex <= 0 {
		return nil
	}
	source := enhancementProfiles[item.EnhancedIndex]
	if len(source) == 0 {
		return nil
	}
	applicable := make([]enhancementSourceRow, 0, len(source))
	for _, row := range source {
		if row.Equip != 0 {
			continue
		}
		name, _, _, _, _ := enhancementBase(item, row.Type)
		if name != "" {
			applicable = append(applicable, row)
		}
	}
	if len(applicable) == 0 {
		return nil
	}
	levels := make([]ItemEnhancementLevel, 0, enhancementMaxLevel+1)
	for level := 0; level <= enhancementMaxLevel; level++ {
		label := "Без усиления"
		if level > 0 {
			label = fmt.Sprintf("+%d", level)
		}
		stats := make([]ItemEnhancementStat, 0, len(applicable))
		for _, row := range applicable {
			name, baseMin, baseMax, base, isRange := enhancementBase(item, row.Type)
			percent := 0.0
			if level > 0 {
				percent = row.Values[level-1]
			}
			stats = append(stats, ItemEnhancementStat{
				Type: row.Type, Name: name, BaseMin: baseMin, BaseMax: baseMax, Base: base,
				Bonus: enhancementBonus(base, percent), Percent: percent, IsRange: isRange,
			})
		}
		levels = append(levels, ItemEnhancementLevel{Level: level, Label: label, Stats: stats})
	}
	return &ItemEnhancement{ProfileID: item.EnhancedIndex, MaxLevel: enhancementMaxLevel, Levels: levels}
}
