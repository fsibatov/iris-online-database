package main

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func isHiddenItem(item *Item) bool {
	return item != nil && (strings.TrimSpace(item.Subcategory) == "---------" || strings.EqualFold(strings.TrimSpace(item.Name), "reserved"))
}

func isCatalogItem(item *Item, query string) bool {
	if item == nil || isHiddenItem(item) || isTitleItem(item) || isTransformationItem(item.ID) {
		return false
	}
	if _, recipe := store.itemRecipes[item.ID]; recipe {
		return false
	}
	return store.itemDuplicates[item.ID] == 0 || strings.TrimSpace(query) == strconv.Itoa(item.ID)
}

func (s *appStore) prepareCatalogDuplicates() {
	referenced := make(map[int]bool)
	for _, server := range s.data.Servers {
		for _, entries := range server.DropLists {
			for _, entry := range entries {
				referenced[entry.ItemID] = true
			}
		}
	}
	for _, drop := range s.data.QuestDrops {
		referenced[drop.ItemID] = true
	}
	for id := range s.questRewards {
		referenced[id] = true
	}
	for id, materials := range s.itemRecipes {
		referenced[id] = true
		for _, material := range materials {
			referenced[material.ItemID] = true
		}
	}
	for id := range s.itemUsedSkills {
		referenced[id] = true
	}
	for _, profiles := range s.chestProfiles {
		for id, profile := range profiles {
			referenced[id] = true
			for _, row := range profile.Rows {
				referenced[row.ItemID] = true
			}
		}
	}
	for _, set := range s.data.ItemSets {
		for _, member := range set.Items {
			referenced[member.ItemID] = true
		}
	}
	for i := range s.data.Items {
		referenced[s.data.Items[i].BuyCurrency] = true
	}

	byName := make(map[string][]*Item)
	for i := range s.data.Items {
		item := &s.data.Items[i]
		if referenced[item.ID] || isHiddenItem(item) || isTitleItem(item) || isTransformationItem(item.ID) {
			continue
		}
		byName[item.Name] = append(byName[item.Name], item)
	}
	s.itemDuplicates = make(map[int]int)
	for _, items := range byName {
		if len(items) < 2 {
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		for index, item := range items {
			value := *item
			value.ID = 0
			for _, previous := range items[:index] {
				candidate := *previous
				candidate.ID = 0
				if reflect.DeepEqual(value, candidate) {
					s.itemDuplicates[item.ID] = previous.ID
					break
				}
			}
		}
	}
}
