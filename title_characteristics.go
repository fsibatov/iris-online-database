package main

import (
	"regexp"
	"strconv"
	"strings"
)

type TitleCharacteristic struct {
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
	Percent bool    `json:"percent"`
	Text    string  `json:"text"`
}

func characteristicFilterKey(name string, percent bool) string {
	name = strings.TrimSpace(name)
	if percent && name != "" {
		return name + " (%)"
	}
	return name
}

var signedTitleValueRE = regexp.MustCompile(`([+\-−])\s*(\d+(?:[.,]\d+)?)\s*(%)?`)

var titleCharacteristicAliases = []struct {
	Needles []string
	Name    string
}{
	{[]string{"все характеристики", "характеристики", "все свойства", "все атрибуты"}, "Все характеристики"},
	{[]string{"предел здоровья", "к пределу здоровья"}, "Максимум здоровья"},
	{[]string{"предел маны", "к пределу маны"}, "Максимум маны"},
	{[]string{"скорость бега", "бег"}, "Скорость бега"},
	{[]string{"скорость атаки"}, "Скорость атаки"},
	{[]string{"скорость чтения"}, "Скорость чтения"},
	{[]string{"физ. уклонение", "физическое уклонение"}, "Физическое уклонение"},
	{[]string{"маг. уклонение", "магическое уклонение"}, "Магическое уклонение"},
	{[]string{"физ. защита", "физическая защита"}, "Физическая защита"},
	{[]string{"маг. защита", "магическая защита"}, "Магическая защита"},
	{[]string{"физ. меткость", "физическая меткость"}, "Физическая меткость"},
	{[]string{"маг. меткость", "магическая меткость"}, "Магическая меткость"},
	{[]string{"физ. ярость", "физическая ярость"}, "Физическая ярость"},
	{[]string{"маг. ярость", "магическая ярость"}, "Магическая ярость"},
	{[]string{"физ. поглощение", "физическое поглощение"}, "Физическое поглощение"},
	{[]string{"маг. поглощение", "магическое поглощение"}, "Магическое поглощение"},
	{[]string{"игнорирование физ. защиты"}, "Игнорирование физической защиты"},
	{[]string{"игнорирование маг. защиты"}, "Игнорирование магической защиты"},
	{[]string{"похищение здоровья"}, "Похищение здоровья"},
	{[]string{"pvp урон", "к pvp урону"}, "PvP-урон"},
	{[]string{"сила атаки", "весь наносимый урон"}, "Наносимый урон"},
	{[]string{"рукопашный урон"}, "Рукопашный урон"},
	{[]string{"сила дистанционных атак"}, "Урон дистанционных атак"},
	{[]string{"маг. урон", "магический урон"}, "Магический урон"},
	{[]string{"исцеление", "объем лечения"}, "Исцеление"},
	{[]string{"вас лечат"}, "Получаемое исцеление"},
	{[]string{"регенерация"}, "Регенерация"},
	{[]string{"поток маны"}, "Поток маны"},
	{[]string{"переносимый вес"}, "Переносимый вес"},
	{[]string{"меткость"}, "Меткость"},
	{[]string{"сила"}, "Сила"},
	{[]string{"ловкость"}, "Ловкость"},
	{[]string{"выносливость"}, "Выносливость"},
	{[]string{"интеллект"}, "Интеллект"},
	{[]string{"мудрость"}, "Мудрость"},
}

func canonicalTitleCharacteristic(line string) string {
	lower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(line, "\u200b", ""), "ё", "е"))
	for _, alias := range titleCharacteristicAliases {
		for _, needle := range alias.Needles {
			if strings.Contains(lower, needle) {
				return alias.Name
			}
		}
	}
	return ""
}

func titleItemCharacteristics(item *Item) []TitleCharacteristic {
	if item == nil {
		return nil
	}
	text := strings.ReplaceAll(strings.ReplaceAll(item.Tooltip, "\\t", " "), "\\n", "\n")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	result := make([]TitleCharacteristic, 0, 4)
	seen := map[string]bool{}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.ReplaceAll(raw, "\u200b", ""))
		match := signedTitleValueRE.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		name := canonicalTitleCharacteristic(line)
		if name == "" {
			continue
		}
		value, err := strconv.ParseFloat(strings.ReplaceAll(match[2], ",", "."), 64)
		if err != nil {
			continue
		}
		if match[1] == "-" || match[1] == "−" {
			value = -value
		}
		percent := match[3] == "%"
		key := name + "\x00" + strconv.FormatBool(percent)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, TitleCharacteristic{Key: characteristicFilterKey(name, percent), Name: name, Value: value, Percent: percent, Text: line})
	}
	return result
}

func titleCharacteristics(title *Title) []TitleCharacteristic {
	if title == nil {
		return nil
	}
	itemIDs := title.ItemIDs
	if len(itemIDs) == 0 && title.ItemID > 0 {
		itemIDs = []int{title.ItemID}
	}
	result := make([]TitleCharacteristic, 0, 4)
	seen := map[string]bool{}
	for _, id := range itemIDs {
		for _, row := range titleItemCharacteristics(store.itemsByID[id]) {
			key := row.Name + "\x00" + strconv.FormatBool(row.Percent) + "\x00" + strconv.FormatFloat(row.Value, 'f', -1, 64)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, row)
		}
	}
	return result
}

func titleCharacteristicValue(title *Title, name string) (float64, bool) {
	found := false
	best := 0.0
	for _, row := range titleCharacteristics(title) {
		if row.Key != name {
			continue
		}
		if !found || row.Value > best {
			best, found = row.Value, true
		}
	}
	return best, found
}

func allTitleCharacteristicNames() []string {
	values := map[string]bool{}
	for i := range store.titles {
		for _, row := range titleCharacteristics(&store.titles[i]) {
			values[row.Key] = true
		}
	}
	return sortedKeys(values)
}
