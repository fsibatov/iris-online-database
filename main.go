package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type Meta struct {
	Title         string `json:"title"`
	Items         int    `json:"items"`
	Monsters      int    `json:"monsters"`
	DataScope     string `json:"dataScope"`
	DropNote      string `json:"dropNote"`
	DataUpdatedAt string `json:"dataUpdatedAt"`
	DropUpdatedAt string `json:"dropUpdatedAt"`
}

type DropListEntry struct {
	ItemID   int     `json:"itemId"`
	Chance   float64 `json:"chance"`
	Quantity int     `json:"quantity"`
}

type GroupRule struct {
	GroupID int     `json:"groupId"`
	Chance  float64 `json:"chance"`
}

type DropSlotRule struct {
	SourceLine       int         `json:"sourceLine"`
	AddAttempt1Count int         `json:"addAttempt1Count,omitempty"`
	AddAttempt1Rate  float64     `json:"addAttempt1Rate,omitempty"`
	AddAttempt2Count int         `json:"addAttempt2Count,omitempty"`
	AddAttempt2Rate  float64     `json:"addAttempt2Rate,omitempty"`
	Choices          []GroupRule `json:"choices"`
}

type WorldRule struct {
	SourceLine       int         `json:"sourceLine"`
	MinLevel         int         `json:"minLevel"`
	MaxLevel         int         `json:"maxLevel"`
	ContextID        int         `json:"contextId"`
	MonsterType      int         `json:"monsterType"`
	AddAttempt1Count int         `json:"addAttempt1Count,omitempty"`
	AddAttempt1Rate  float64     `json:"addAttempt1Rate,omitempty"`
	AddAttempt2Count int         `json:"addAttempt2Count,omitempty"`
	AddAttempt2Rate  float64     `json:"addAttempt2Rate,omitempty"`
	Groups           []GroupRule `json:"groups"`
}

type ServerData struct {
	Name                  string                     `json:"name"`
	QuestDrops            int                        `json:"questDrops"`
	DirectRulesCount      int                        `json:"directRulesCount"`
	DirectMonsters        int                        `json:"directMonsters"`
	DirectDropEntries     int                        `json:"directDropEntries"`
	DirectDropSlots       int                        `json:"directDropSlots"`
	WorldRulesCount       int                        `json:"worldRulesCount"`
	DropListGroups        int                        `json:"dropListGroups"`
	DropListEntries       int                        `json:"dropListEntries"`
	ChangeProfiles        int                        `json:"changeProfiles"`
	ChangedChangeProfiles int                        `json:"changedChangeProfiles"`
	DirectDropsUpdatedAt  string                     `json:"directDropsUpdatedAt"`
	DropListsUpdatedAt    string                     `json:"dropListsUpdatedAt"`
	WorldDropsUpdatedAt   string                     `json:"worldDropsUpdatedAt"`
	DropLists             map[string][]DropListEntry `json:"dropLists"`
	DirectSlots           map[string][]DropSlotRule  `json:"directSlots"`
	WorldRules            []WorldRule                `json:"worldRules"`
}

type StatOption struct {
	Type  int `json:"type"`
	Value int `json:"value"`
}

type ItemEffectSpec struct {
	Name         string `json:"name"`
	Percent      bool   `json:"percent"`
	TooltipIndex int    `json:"tooltipIndex"`
}

type ItemSetMember struct {
	ItemID int    `json:"itemId"`
	Item   string `json:"item"`
}

type ItemSetActiveEffect struct {
	ID           int     `json:"id"`
	TooltipIndex int     `json:"tooltipIndex"`
	State        int     `json:"state"`
	Chance       float64 `json:"chance"`
	Text         string  `json:"text"`
}

type ItemSetEffect struct {
	Required int                  `json:"required"`
	Options  []StatOption         `json:"options"`
	Active   *ItemSetActiveEffect `json:"active,omitempty"`
}

type ItemSet struct {
	Name    string          `json:"name,omitempty"`
	Items   []ItemSetMember `json:"items"`
	Effects []ItemSetEffect `json:"effects"`
}

type itemSetSupplement struct {
	SchemaVersion int                `json:"schemaVersion"`
	Sets          map[string]ItemSet `json:"sets"`
}

type itemAbilityPatch struct {
	PhysicalDefense         *int          `json:"physicalDefense,omitempty"`
	MagicDefense            *int          `json:"magicDefense,omitempty"`
	AttackRange             *int          `json:"attackRange,omitempty"`
	AttackSpeed             *int          `json:"attackSpeed,omitempty"`
	Cooldown                *int          `json:"cooldown,omitempty"`
	PhysicalMin             *int          `json:"physicalMin,omitempty"`
	PhysicalMax             *int          `json:"physicalMax,omitempty"`
	MagicMin                *int          `json:"magicMin,omitempty"`
	MagicMax                *int          `json:"magicMax,omitempty"`
	Heal                    *int          `json:"heal,omitempty"`
	Options                 *[]StatOption `json:"options,omitempty"`
	AbilityDescription      string        `json:"abilityDescription,omitempty"`
	AbilityDescriptionIndex int           `json:"abilityDescriptionIndex,omitempty"`
	DefenseType             int           `json:"defenseType,omitempty"`
	RangeType               int           `json:"rangeType,omitempty"`
	TargetType              int           `json:"targetType,omitempty"`
	UseRange                int           `json:"useRange,omitempty"`
	GroupTime               int           `json:"groupTime,omitempty"`
	InfluenceIndex          int           `json:"influenceIndex,omitempty"`
	ActiveIndex             int           `json:"activeIndex,omitempty"`
	EffectDurationMs        int           `json:"effectDurationMs,omitempty"`
	NameIndex               int           `json:"nameIndex,omitempty"`
	TooltipIndex            int           `json:"tooltipIndex,omitempty"`
	AbilityIndex            int           `json:"abilityIndex,omitempty"`
	CardIndex               int           `json:"cardIndex,omitempty"`
	UseMapType              int           `json:"useMapType,omitempty"`
	MakeSkill               int           `json:"makeSkill,omitempty"`
	MakeSkillExp            int           `json:"makeSkillExp,omitempty"`
	GuildUse                int           `json:"guildUse,omitempty"`
	LimitMapTypeRaw         int           `json:"limitMapTypeRaw,omitempty"`
	LimitValueRaw           int           `json:"limitValueRaw,omitempty"`
	LimitExtraRaw           int           `json:"limitExtraRaw,omitempty"`
	KindOf                  int           `json:"kindOf,omitempty"`
	EventType               int           `json:"eventType,omitempty"`
	BuyCurrency             int           `json:"buyCurrency,omitempty"`
	BuyPrice                int           `json:"buyPrice,omitempty"`
	MaxInventory            int           `json:"maxInventory,omitempty"`
	TermSet                 int           `json:"termSet,omitempty"`
	TermDuration            int           `json:"termDuration,omitempty"`
	PrintableFlag           int           `json:"printableFlag,omitempty"`
	LimitIndex              int           `json:"limitIndex,omitempty"`
	TarotIndex              int           `json:"tarotIndex,omitempty"`
	SpreadIndex             int           `json:"spreadIndex,omitempty"`
	DegradationIndex        int           `json:"degradationIndex,omitempty"`
	CardSlotIndex           int           `json:"cardSlotIndex,omitempty"`
	EnhanceProbabilityIndex int           `json:"enhanceProbabilityIndex,omitempty"`
	EnhancedIndex           int           `json:"enhancedIndex,omitempty"`
	ReinforcingIndex        int           `json:"reinforcingIndex,omitempty"`
	ChangeIndex             int           `json:"changeIndex,omitempty"`
	TitleIndex              int           `json:"titleIndex,omitempty"`
	ModelIndex              int           `json:"modelIndex,omitempty"`
	ModelLeftIndex          int           `json:"modelLeftIndex,omitempty"`
	Gwipyosi                int           `json:"gwipyosi,omitempty"`
}

type itemAbilitySupplement struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Items         map[string]itemAbilityPatch `json:"items"`
}

type Item struct {
	ID                      int          `json:"id"`
	Name                    string       `json:"name"`
	Tooltip                 string       `json:"tooltip"`
	MainCategoryID          int          `json:"mainCategoryId"`
	MainCategory            string       `json:"mainCategory"`
	MiddleCategoryID        int          `json:"middleCategoryId"`
	MiddleCategory          string       `json:"middleCategory"`
	SubCategoryID           int          `json:"subCategoryId"`
	SubCategory             string       `json:"subCategory"`
	Category                string       `json:"category"`
	Subcategory             string       `json:"subcategory"`
	TypeLine                string       `json:"typeLine"`
	QualityID               int          `json:"qualityId"`
	Quality                 string       `json:"quality"`
	Weight                  int          `json:"weight"`
	Capacity                int          `json:"capacity"`
	SellType                int          `json:"sellType"`
	Price                   int          `json:"price"`
	MaxStack                int          `json:"maxStack"`
	Exchange                int          `json:"exchange"`
	Seal                    int          `json:"seal"`
	CardSlots               []string     `json:"cardSlots"`
	SetIndex                int          `json:"setIndex"`
	IconIndex               int          `json:"iconIndex"`
	Race                    int          `json:"race"`
	RaceName                string       `json:"raceName"`
	Gender                  int          `json:"gender"`
	GenderName              string       `json:"genderName"`
	Job1                    int          `json:"job1"`
	Job1Name                string       `json:"job1Name"`
	Job2                    int          `json:"job2"`
	Job2Name                string       `json:"job2Name"`
	MinLevel                int          `json:"minLevel"`
	MaxLevel                int          `json:"maxLevel"`
	PhysicalDefense         int          `json:"physicalDefense"`
	MagicDefense            int          `json:"magicDefense"`
	AttackRange             int          `json:"attackRange"`
	AttackSpeed             int          `json:"attackSpeed"`
	Cooldown                int          `json:"cooldown"`
	PhysicalMin             int          `json:"physicalMin"`
	PhysicalMax             int          `json:"physicalMax"`
	MagicMin                int          `json:"magicMin"`
	MagicMax                int          `json:"magicMax"`
	Heal                    int          `json:"heal"`
	Options                 []StatOption `json:"options"`
	AbilityDescription      string       `json:"abilityDescription,omitempty"`
	AbilityDescriptionIndex int          `json:"abilityDescriptionIndex,omitempty"`
	DefenseType             int          `json:"defenseType,omitempty"`
	RangeType               int          `json:"rangeType,omitempty"`
	TargetType              int          `json:"targetType,omitempty"`
	UseRange                int          `json:"useRange,omitempty"`
	GroupTime               int          `json:"groupTime,omitempty"`
	InfluenceIndex          int          `json:"influenceIndex,omitempty"`
	ActiveIndex             int          `json:"activeIndex,omitempty"`
	EffectDurationMs        int          `json:"effectDurationMs,omitempty"`
	NameIndex               int          `json:"nameIndex,omitempty"`
	TooltipIndex            int          `json:"tooltipIndex,omitempty"`
	AbilityIndex            int          `json:"abilityIndex,omitempty"`
	CardIndex               int          `json:"cardIndex,omitempty"`
	UseMapType              int          `json:"useMapType,omitempty"`
	MakeSkill               int          `json:"makeSkill,omitempty"`
	MakeSkillExp            int          `json:"makeSkillExp,omitempty"`
	GuildUse                int          `json:"guildUse,omitempty"`
	LimitMapTypeRaw         int          `json:"limitMapTypeRaw,omitempty"`
	LimitValueRaw           int          `json:"limitValueRaw,omitempty"`
	LimitExtraRaw           int          `json:"limitExtraRaw,omitempty"`
	KindOf                  int          `json:"kindOf,omitempty"`
	EventType               int          `json:"eventType,omitempty"`
	BuyCurrency             int          `json:"buyCurrency,omitempty"`
	BuyPrice                int          `json:"buyPrice,omitempty"`
	MaxInventory            int          `json:"maxInventory,omitempty"`
	TermSet                 int          `json:"termSet,omitempty"`
	TermDuration            int          `json:"termDuration,omitempty"`
	PrintableFlag           int          `json:"printableFlag,omitempty"`
	LimitIndex              int          `json:"limitIndex,omitempty"`
	TarotIndex              int          `json:"tarotIndex,omitempty"`
	SpreadIndex             int          `json:"spreadIndex,omitempty"`
	DegradationIndex        int          `json:"degradationIndex,omitempty"`
	CardSlotIndex           int          `json:"cardSlotIndex,omitempty"`
	EnhanceProbabilityIndex int          `json:"enhanceProbabilityIndex,omitempty"`
	EnhancedIndex           int          `json:"enhancedIndex,omitempty"`
	ReinforcingIndex        int          `json:"reinforcingIndex,omitempty"`
	ChangeIndex             int          `json:"changeIndex,omitempty"`
	TitleIndex              int          `json:"titleIndex,omitempty"`
	ModelIndex              int          `json:"modelIndex,omitempty"`
	ModelLeftIndex          int          `json:"modelLeftIndex,omitempty"`
	Gwipyosi                int          `json:"gwipyosi,omitempty"`
}

type ItemRecipeMaterial struct {
	ItemID   int    `json:"itemId"`
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

type itemRecipeMaterialSource struct {
	ItemID   int `json:"itemId"`
	Quantity int `json:"quantity"`
}

type itemRecipeSupplement struct {
	SchemaVersion int                                   `json:"schemaVersion"`
	Recipes       map[string][]itemRecipeMaterialSource `json:"recipes"`
}

type chestContentSourceRow struct {
	ItemID    int `json:"itemId"`
	Quantity  int `json:"quantity"`
	Enhanced  int `json:"enhanced"`
	Threshold int `json:"threshold"`
	Position  int `json:"position"`
}

type chestProfileSource struct {
	DrawCount int                     `json:"drawCount"`
	Rows      []chestContentSourceRow `json:"rows"`
}

type chestServerSupplement struct {
	Profiles map[string]chestProfileSource `json:"profiles"`
}

type chestContentSupplement struct {
	SchemaVersion int                              `json:"schemaVersion"`
	Servers       map[string]chestServerSupplement `json:"servers"`
}

type ChestVariant struct {
	Quantity    int     `json:"quantity"`
	Enhanced    int     `json:"enhanced,omitempty"`
	Chance      float64 `json:"chance,omitempty"`
	ChanceKnown bool    `json:"chanceKnown"`
}

type ChestContentItem struct {
	ItemID      int            `json:"itemId"`
	Item        string         `json:"item"`
	ItemKnown   bool           `json:"itemKnown"`
	Chance      float64        `json:"chance,omitempty"`
	ChanceKnown bool           `json:"chanceKnown"`
	Variants    []ChestVariant `json:"variants,omitempty"`
}

type ChestContents struct {
	ChestID   int                `json:"chestId"`
	Chest     string             `json:"chest"`
	DrawCount int                `json:"drawCount"`
	Items     []ChestContentItem `json:"items"`
}

type WorldSourceMonster struct {
	MonsterID int     `json:"monsterId"`
	Monster   string  `json:"monster"`
	Level     int     `json:"level,omitempty"`
	Chance    float64 `json:"chance"`
}

type Monster struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	Note               string  `json:"note"`
	JobID              int     `json:"jobId"`
	Kind               int     `json:"kind"`
	CategoryID         int     `json:"categoryId"`
	Category           string  `json:"category"`
	Type               int     `json:"type"`
	TypeName           string  `json:"typeName"`
	Level              int     `json:"level"`
	HP                 int     `json:"hp"`
	MP                 int     `json:"mp"`
	Exp                int     `json:"exp"`
	MoneyBonus         int     `json:"moneyBonus"`
	Defense            int     `json:"defense"`
	MagicDefense       int     `json:"magicDefense"`
	Hit                int     `json:"hit"`
	Evasion            int     `json:"evasion"`
	CriticalDefense    int     `json:"criticalDefense"`
	ViewRange          int     `json:"viewRange"`
	Importance         int     `json:"importance"`
	Scale              float64 `json:"scale"`
	AttackRadius       float64 `json:"attackRadius"`
	WalkSpeed          int     `json:"walkSpeed"`
	RunSpeed           int     `json:"runSpeed"`
	Aggressive         bool    `json:"aggressive"`
	FollowRange        int     `json:"followRange"`
	Recovery           int     `json:"recovery"`
	NameIndex          int     `json:"nameIndex,omitempty"`
	NoteIndex          int     `json:"noteIndex,omitempty"`
	NameHeight         int     `json:"nameHeight,omitempty"`
	SourceFlag         int     `json:"sourceFlag,omitempty"`
	EffectScale        float64 `json:"effectScale,omitempty"`
	FreeMoveRange      int     `json:"freeMoveRange,omitempty"`
	ActionStopRatio    int     `json:"actionStopRatio,omitempty"`
	ActionWalkRatio    int     `json:"actionWalkRatio,omitempty"`
	ActionRunRatio     int     `json:"actionRunRatio,omitempty"`
	ActionStopTime     int     `json:"actionStopTime,omitempty"`
	ChangeMonsterCheck int     `json:"changeMonsterCheck,omitempty"`
	FollowTime         int     `json:"followTime,omitempty"`
	EscapeType         int     `json:"escapeType,omitempty"`
	EscapePercent      int     `json:"escapePercent,omitempty"`
	RecoveryTime       int     `json:"recoveryTime,omitempty"`
}

type monsterDetailPatch struct {
	NameIndex          int     `json:"nameIndex"`
	NoteIndex          int     `json:"noteIndex"`
	NameHeight         int     `json:"nameHeight"`
	SourceFlag         int     `json:"sourceFlag"`
	EffectScale        float64 `json:"effectScale"`
	FreeMoveRange      int     `json:"freeMoveRange"`
	ActionStopRatio    int     `json:"actionStopRatio"`
	ActionWalkRatio    int     `json:"actionWalkRatio"`
	ActionRunRatio     int     `json:"actionRunRatio"`
	ActionStopTime     int     `json:"actionStopTime"`
	ChangeMonsterCheck int     `json:"changeMonsterCheck"`
	FollowTime         int     `json:"followTime"`
	EscapeType         int     `json:"escapeType"`
	EscapePercent      int     `json:"escapePercent"`
	RecoveryTime       int     `json:"recoveryTime"`
}

type monsterDetailSupplement struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Monsters      map[string]monsterDetailPatch `json:"monsters"`
}

type Drop struct {
	QuestID          int     `json:"questId"`
	Quest            string  `json:"quest"`
	MonsterID        int     `json:"monsterId"`
	Monster          string  `json:"monster"`
	ItemID           int     `json:"itemId"`
	Item             string  `json:"item"`
	Chance           float64 `json:"chance"`
	Source           string  `json:"source"`
	Context          string  `json:"context"`
	GroupTitle       string  `json:"groupTitle"`
	GroupChance      float64 `json:"groupChance"`
	GroupChanceKnown bool    `json:"groupChanceKnown"`
	ItemChance       float64 `json:"itemChance"`
	EffectiveChance  float64 `json:"effectiveChance"`
	Quantity         int     `json:"quantity"`
}

type GameData struct {
	Meta        Meta                      `json:"meta"`
	EffectSpecs map[string]ItemEffectSpec `json:"effectSpecs"`
	QuestDrops  []Drop                    `json:"questDrops"`
	Servers     map[string]ServerData     `json:"servers"`
	Items       []Item                    `json:"items"`
	ItemSets    map[string]ItemSet        `json:"itemSets"`
	Monsters    []Monster                 `json:"monsters"`
}

type DropItem struct {
	ItemID              int     `json:"itemId"`
	Item                string  `json:"item"`
	Chance              float64 `json:"chance"`
	BaseSelectionChance float64 `json:"baseSelectionChance"`
	BaseAttemptChance   float64 `json:"baseAttemptChance,omitempty"`
	Quantity            int     `json:"quantity"`
	Position            int     `json:"position"`
}

type DropChoice struct {
	ID                  string     `json:"id"`
	GroupID             int        `json:"groupId"`
	Title               string     `json:"title"`
	Chance              float64    `json:"chance"`
	BaseSelectionChance float64    `json:"baseSelectionChance"`
	Items               []DropItem `json:"items"`
	ItemChanceTotal     float64    `json:"itemChanceTotal"`
	ItemEmptyChance     float64    `json:"itemEmptyChance"`
	ItemChanceOverflow  bool       `json:"itemChanceOverflow"`
}

type DropSlot struct {
	ID               string       `json:"id"`
	Title            string       `json:"title"`
	Source           string       `json:"source"`
	Context          string       `json:"context"`
	SlotNumber       int          `json:"slotNumber"`
	SourceLine       int          `json:"sourceLine"`
	AddAttempt1Count int          `json:"addAttempt1Count,omitempty"`
	AddAttempt1Rate  float64      `json:"addAttempt1Rate,omitempty"`
	AddAttempt2Count int          `json:"addAttempt2Count,omitempty"`
	AddAttempt2Rate  float64      `json:"addAttempt2Rate,omitempty"`
	ChanceTotal      float64      `json:"chanceTotal"`
	EmptyChance      float64      `json:"emptyChance"`
	ChanceOverflow   bool         `json:"chanceOverflow"`
	Choices          []DropChoice `json:"choices"`
	Note             string       `json:"note"`
}

type DropGroup struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Source      string     `json:"source"`
	Context     string     `json:"context"`
	Chance      float64    `json:"chance"`
	ChanceKnown bool       `json:"chanceKnown"`
	Items       []DropItem `json:"items"`
	Note        string     `json:"note"`
}

type ItemDrop struct {
	ItemID            int            `json:"itemId"`
	MonsterID         int            `json:"monsterId"`
	Monster           string         `json:"monster"`
	MonsterLevel      int            `json:"monsterLevel"`
	ContainerID       int            `json:"containerId,omitempty"`
	Container         string         `json:"container,omitempty"`
	Variants          []ChestVariant `json:"variants,omitempty"`
	ChanceKnown       bool           `json:"chanceKnown,omitempty"`
	QuestID           int            `json:"questId"`
	Quest             string         `json:"quest"`
	Source            string         `json:"source"`
	Context           string         `json:"context"`
	GroupTitle        string         `json:"groupTitle"`
	GroupID           int            `json:"groupId"`
	GroupChance       float64        `json:"groupChance"`
	GroupChanceKnown  bool           `json:"groupChanceKnown"`
	GroupBaseChance   float64        `json:"groupBaseChance,omitempty"`
	ItemChance        float64        `json:"itemChance"`
	ItemBaseChance    float64        `json:"itemBaseChance,omitempty"`
	BaseAttemptChance float64        `json:"baseAttemptChance,omitempty"`
	Quantity          int            `json:"quantity"`
	SlotNumber        int            `json:"slotNumber"`
	SlotTitle         string         `json:"slotTitle"`
	ChoicePosition    int            `json:"choicePosition"`
	ItemPosition      int            `json:"itemPosition"`
	SourceLine        int            `json:"sourceLine"`
	ChanceOverflow    bool           `json:"chanceOverflow"`
	ItemOverflow      bool           `json:"itemOverflow"`
}

type runtimeData struct {
	server           ServerData
	monsterIDs       map[int]struct{}
	resolved         map[int][]DropItem
	questByItem      map[int][]ItemDrop
	chestProfiles    map[int]chestProfileSource
	chestByItem      map[int][]ItemDrop
	knownSourceItems map[int]struct{}
}

type runtimeSlot struct {
	once          sync.Once
	server        ServerData
	monsterIDs    map[int]struct{}
	chestProfiles map[int]chestProfileSource
	value         *runtimeData
}

type monsterPresenceSupplement struct {
	SchemaVersion int              `json:"schemaVersion"`
	Servers       map[string][]int `json:"servers"`
}

type searchDocument struct {
	Literal string
	Stems   string
}

type appStore struct {
	data             GameData
	itemsByID        map[int]*Item
	monstersByID     map[int]*Monster
	itemNames        map[int]string
	itemSearch       []searchDocument
	monsterSearch    []searchDocument
	monsterTypeNames map[int]string
	categoryItems    map[string]int
	itemRecipes      map[int][]itemRecipeMaterialSource
	chestProfiles    map[string]map[int]chestProfileSource
	monsterPresence  map[string]map[int]struct{}
	runtimes         map[string]*runtimeSlot
}

var store appStore
var loadOnce sync.Once
var loadErr error

func ensureLoaded() error {
	loadOnce.Do(func() {
		raw, err := embedded.ReadFile("assets/game_data.json.gz")
		if err != nil {
			loadErr = err
			return
		}
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			loadErr = err
			return
		}
		defer gz.Close()
		decoder := json.NewDecoder(gz)
		if err := decoder.Decode(&store.data); err != nil {
			loadErr = err
			return
		}
		if err := mergeItemAbilitySupplement(); err != nil {
			loadErr = err
			return
		}
		if err := mergeMonsterDetailSupplement(); err != nil {
			loadErr = err
			return
		}
		if err := mergeSetSupplement(); err != nil {
			loadErr = err
			return
		}
		if err := mergeRecipeSupplement(); err != nil {
			loadErr = err
			return
		}
		if err := mergeChestContentSupplement(); err != nil {
			loadErr = err
			return
		}
		if err := mergeMonsterPresenceSupplement(); err != nil {
			loadErr = err
			return
		}
		store.itemsByID = make(map[int]*Item, len(store.data.Items))
		store.monstersByID = make(map[int]*Monster, len(store.data.Monsters))
		store.itemNames = make(map[int]string, len(store.data.Items))
		store.itemSearch = make([]searchDocument, len(store.data.Items))
		store.monsterSearch = make([]searchDocument, len(store.data.Monsters))
		store.monsterTypeNames = make(map[int]string)
		store.categoryItems = make(map[string]int)
		for i := range store.data.Items {
			item := &store.data.Items[i]
			store.itemsByID[item.ID] = item
			store.itemNames[item.ID] = item.Name
			store.itemSearch[i] = newSearchDocument(fmt.Sprintf("%d %s %s %s %s %s", item.ID, item.Name, item.TypeLine, item.Category, item.Subcategory, item.Quality))
			store.categoryItems[item.Category]++
		}
		for i := range store.data.Monsters {
			monster := &store.data.Monsters[i]
			store.monstersByID[monster.ID] = monster
			store.monsterSearch[i] = newSearchDocument(fmt.Sprintf("%d %s %s %s", monster.ID, monster.Name, monster.Category, monster.TypeName))
			if _, exists := store.monsterTypeNames[monster.Type]; !exists && strings.TrimSpace(monster.TypeName) != "" {
				store.monsterTypeNames[monster.Type] = strings.TrimSpace(monster.TypeName)
			}
		}
		store.runtimes = make(map[string]*runtimeSlot, len(store.data.Servers))
		for key, server := range store.data.Servers {
			store.runtimes[key] = &runtimeSlot{server: server, chestProfiles: store.chestProfiles[key], monsterIDs: store.monsterPresence[key]}
		}
	})
	return loadErr
}

func mergeItemAbilitySupplement() error {
	raw, err := embedded.ReadFile("assets/item_abilities.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать дополнительные характеристики предметов: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть дополнительные характеристики предметов: %w", err)
	}
	defer gz.Close()
	var supplement itemAbilitySupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать дополнительные характеристики предметов: %w", err)
	}
	if supplement.SchemaVersion != 3 {
		return fmt.Errorf("неподдерживаемая версия дополнительных характеристик предметов: %d", supplement.SchemaVersion)
	}
	byID := make(map[int]*Item, len(store.data.Items))
	for index := range store.data.Items {
		item := &store.data.Items[index]
		byID[item.ID] = item
	}
	for key, patch := range supplement.Items {
		id, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("некорректный ID предмета в дополнительных характеристиках: %q", key)
		}
		item := byID[id]
		if item == nil {
			return fmt.Errorf("предмет %d из дополнительных характеристик отсутствует в основной базе", id)
		}
		if patch.PhysicalDefense != nil {
			item.PhysicalDefense = *patch.PhysicalDefense
		}
		if patch.MagicDefense != nil {
			item.MagicDefense = *patch.MagicDefense
		}
		if patch.AttackRange != nil {
			item.AttackRange = *patch.AttackRange
		}
		if patch.AttackSpeed != nil {
			item.AttackSpeed = *patch.AttackSpeed
		}
		if patch.Cooldown != nil {
			item.Cooldown = *patch.Cooldown
		}
		if patch.PhysicalMin != nil {
			item.PhysicalMin = *patch.PhysicalMin
		}
		if patch.PhysicalMax != nil {
			item.PhysicalMax = *patch.PhysicalMax
		}
		if patch.MagicMin != nil {
			item.MagicMin = *patch.MagicMin
		}
		if patch.MagicMax != nil {
			item.MagicMax = *patch.MagicMax
		}
		if patch.Heal != nil {
			item.Heal = *patch.Heal
		}
		if patch.Options != nil {
			item.Options = append([]StatOption(nil), (*patch.Options)...)
		}
		item.AbilityDescription = patch.AbilityDescription
		item.AbilityDescriptionIndex = patch.AbilityDescriptionIndex
		item.DefenseType = patch.DefenseType
		item.RangeType = patch.RangeType
		item.TargetType = patch.TargetType
		item.UseRange = patch.UseRange
		item.GroupTime = patch.GroupTime
		item.InfluenceIndex = patch.InfluenceIndex
		item.ActiveIndex = patch.ActiveIndex
		item.EffectDurationMs = patch.EffectDurationMs
		item.NameIndex = patch.NameIndex
		item.TooltipIndex = patch.TooltipIndex
		item.AbilityIndex = patch.AbilityIndex
		item.CardIndex = patch.CardIndex
		item.UseMapType = patch.UseMapType
		item.MakeSkill = patch.MakeSkill
		item.MakeSkillExp = patch.MakeSkillExp
		item.GuildUse = patch.GuildUse
		item.LimitMapTypeRaw = patch.LimitMapTypeRaw
		item.LimitValueRaw = patch.LimitValueRaw
		item.LimitExtraRaw = patch.LimitExtraRaw
		item.KindOf = patch.KindOf
		item.EventType = patch.EventType
		item.BuyCurrency = patch.BuyCurrency
		item.BuyPrice = patch.BuyPrice
		item.MaxInventory = patch.MaxInventory
		item.TermSet = patch.TermSet
		item.TermDuration = patch.TermDuration
		item.PrintableFlag = patch.PrintableFlag
		item.LimitIndex = patch.LimitIndex
		item.TarotIndex = patch.TarotIndex
		item.SpreadIndex = patch.SpreadIndex
		item.DegradationIndex = patch.DegradationIndex
		item.CardSlotIndex = patch.CardSlotIndex
		item.EnhanceProbabilityIndex = patch.EnhanceProbabilityIndex
		item.EnhancedIndex = patch.EnhancedIndex
		item.ReinforcingIndex = patch.ReinforcingIndex
		item.ChangeIndex = patch.ChangeIndex
		item.TitleIndex = patch.TitleIndex
		item.ModelIndex = patch.ModelIndex
		item.ModelLeftIndex = patch.ModelLeftIndex
		item.Gwipyosi = patch.Gwipyosi
	}
	return nil
}

func mergeMonsterDetailSupplement() error {
	raw, err := embedded.ReadFile("assets/monster_details.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать дополнительные данные монстров: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть дополнительные данные монстров: %w", err)
	}
	defer gz.Close()
	var supplement monsterDetailSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать дополнительные данные монстров: %w", err)
	}
	if supplement.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия дополнительных данных монстров: %d", supplement.SchemaVersion)
	}
	byID := make(map[int]*Monster, len(store.data.Monsters))
	for index := range store.data.Monsters {
		monster := &store.data.Monsters[index]
		byID[monster.ID] = monster
	}
	for key, patch := range supplement.Monsters {
		id, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("некорректный ID монстра в дополнительных данных: %q", key)
		}
		monster := byID[id]
		if monster == nil {
			return fmt.Errorf("монстр %d из дополнительных данных отсутствует в основной базе", id)
		}
		monster.NameIndex = patch.NameIndex
		monster.NoteIndex = patch.NoteIndex
		monster.NameHeight = patch.NameHeight
		monster.SourceFlag = patch.SourceFlag
		monster.EffectScale = patch.EffectScale
		monster.FreeMoveRange = patch.FreeMoveRange
		monster.ActionStopRatio = patch.ActionStopRatio
		monster.ActionWalkRatio = patch.ActionWalkRatio
		monster.ActionRunRatio = patch.ActionRunRatio
		monster.ActionStopTime = patch.ActionStopTime
		monster.ChangeMonsterCheck = patch.ChangeMonsterCheck
		monster.FollowTime = patch.FollowTime
		monster.EscapeType = patch.EscapeType
		monster.EscapePercent = patch.EscapePercent
		monster.RecoveryTime = patch.RecoveryTime
	}
	return nil
}

func mergeSetSupplement() error {
	raw, err := embedded.ReadFile("assets/set_effects.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать дополнительные данные комплектов: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть дополнительные данные комплектов: %w", err)
	}
	defer gz.Close()
	var supplement itemSetSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать дополнительные данные комплектов: %w", err)
	}
	if supplement.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия дополнительных данных комплектов: %d", supplement.SchemaVersion)
	}
	for key, extra := range supplement.Sets {
		set := store.data.ItemSets[key]
		if strings.TrimSpace(extra.Name) != "" {
			set.Name = extra.Name
		}

		if len(extra.Effects) != 0 {
			set.Effects = extra.Effects
		}
		store.data.ItemSets[key] = set
	}
	return nil
}

func mergeRecipeSupplement() error {
	raw, err := embedded.ReadFile("assets/item_recipes.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать материалы рецептов: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть материалы рецептов: %w", err)
	}
	defer gz.Close()
	var supplement itemRecipeSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать материалы рецептов: %w", err)
	}
	if supplement.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия материалов рецептов: %d", supplement.SchemaVersion)
	}
	store.itemRecipes = make(map[int][]itemRecipeMaterialSource, len(supplement.Recipes))
	for key, materials := range supplement.Recipes {
		id, err := strconv.Atoi(key)
		if err != nil || id <= 0 {
			return fmt.Errorf("некорректный ID рецепта: %q", key)
		}
		store.itemRecipes[id] = append([]itemRecipeMaterialSource(nil), materials...)
	}
	return nil
}

func mergeChestContentSupplement() error {
	raw, err := embedded.ReadFile("assets/chest_contents.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать содержимое сундуков: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть содержимое сундуков: %w", err)
	}
	defer gz.Close()
	var supplement chestContentSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать содержимое сундуков: %w", err)
	}
	if supplement.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия содержимого сундуков: %d", supplement.SchemaVersion)
	}
	store.chestProfiles = make(map[string]map[int]chestProfileSource, len(supplement.Servers))
	for serverKey, server := range supplement.Servers {
		profiles := make(map[int]chestProfileSource, len(server.Profiles))
		for key, profile := range server.Profiles {
			chestID, err := strconv.Atoi(key)
			if err != nil || chestID <= 0 {
				return fmt.Errorf("некорректный ID сундука: %q", key)
			}
			if store.itemsByID != nil && store.itemsByID[chestID] == nil {
				return fmt.Errorf("сундук %d отсутствует в основной базе", chestID)
			}
			if profile.DrawCount < 0 || profile.DrawCount > 40 {
				return fmt.Errorf("некорректное количество выборов сундука %d: %d", chestID, profile.DrawCount)
			}
			rows := append([]chestContentSourceRow(nil), profile.Rows...)
			positions := make(map[int]bool, len(rows))
			for i, row := range rows {
				if row.ItemID <= 0 || row.Quantity <= 0 || row.Enhanced < 0 || row.Threshold <= 0 || row.Position <= 0 {
					return fmt.Errorf("некорректная строка содержимого сундука %d, позиция %d", chestID, i+1)
				}
				if positions[row.Position] {
					return fmt.Errorf("повторяющаяся позиция %d в содержимом сундука %d", row.Position, chestID)
				}
				positions[row.Position] = true
			}
			if len(rows) > 0 && profile.DrawCount <= 0 {
				return fmt.Errorf("сундук %d содержит предметы, но количество выборов равно %d", chestID, profile.DrawCount)
			}
			profiles[chestID] = chestProfileSource{DrawCount: profile.DrawCount, Rows: rows}
		}
		store.chestProfiles[normalizeServerDataKey(serverKey)] = profiles
	}
	return nil
}

func mergeMonsterPresenceSupplement() error {
	raw, err := embedded.ReadFile("assets/monster_presence.json.gz")
	if err != nil {
		return fmt.Errorf("не удалось прочитать список монстров серверов: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не удалось открыть список монстров серверов: %w", err)
	}
	defer gz.Close()
	var supplement monsterPresenceSupplement
	if err := json.NewDecoder(gz).Decode(&supplement); err != nil {
		return fmt.Errorf("не удалось разобрать список монстров серверов: %w", err)
	}
	if supplement.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия списка монстров серверов: %d", supplement.SchemaVersion)
	}
	known := make(map[int]struct{}, len(store.data.Monsters))
	for index := range store.data.Monsters {
		known[store.data.Monsters[index].ID] = struct{}{}
	}
	store.monsterPresence = make(map[string]map[int]struct{}, len(supplement.Servers))
	for rawKey, ids := range supplement.Servers {
		key := normalizeServerDataKey(rawKey)
		if _, exists := store.monsterPresence[key]; exists {
			return fmt.Errorf("повторяющийся сервер в списке монстров: %q", key)
		}
		visible := make(map[int]struct{}, len(ids))
		for _, id := range ids {
			if id <= 0 {
				return fmt.Errorf("некорректный ID монстра для сервера %s: %d", key, id)
			}
			if _, ok := known[id]; !ok {
				return fmt.Errorf("монстр %d из списка сервера %s отсутствует в основной базе", id, key)
			}
			if _, duplicate := visible[id]; duplicate {
				return fmt.Errorf("повторяющийся ID монстра %d для сервера %s", id, key)
			}
			visible[id] = struct{}{}
		}
		if len(visible) == 0 {
			return fmt.Errorf("пустой список монстров для сервера %s", key)
		}
		store.monsterPresence[key] = visible
	}
	for key := range store.data.Servers {
		key = normalizeServerDataKey(key)
		if len(store.monsterPresence[key]) == 0 {
			return fmt.Errorf("для сервера %s отсутствует список монстров", key)
		}
	}
	return nil
}

func monsterVisible(rt *runtimeData, monsterID int) bool {
	if rt == nil || monsterID <= 0 {
		return false
	}
	_, ok := rt.monsterIDs[monsterID]
	return ok
}

func normalizeServerDataKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "or" {
		return "original"
	}
	return key
}

func itemRecipeMaterials(itemID int) []ItemRecipeMaterial {
	source := store.itemRecipes[itemID]
	if len(source) == 0 {
		return nil
	}
	result := make([]ItemRecipeMaterial, 0, len(source))
	for _, material := range source {
		name := store.itemNames[material.ItemID]
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Неизвестный предмет (ID %d)", material.ItemID)
		}
		result = append(result, ItemRecipeMaterial{ItemID: material.ItemID, Item: name, Quantity: max(1, material.Quantity)})
	}
	return result
}

func combination(n, k int) float64 {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n-k {
		k = n - k
	}
	result := 1.0
	for i := 1; i <= k; i++ {
		result *= float64(n-k+i) / float64(i)
	}
	return result
}

type chestTier struct {
	Threshold int
	Chance    float64
	Rows      []chestContentSourceRow
}

func chestProbabilityKnown(profile chestProfileSource) bool {
	if len(profile.Rows) == 0 || profile.DrawCount <= 0 {
		return false
	}
	tierSizes := make(map[int]int)
	maxThreshold := 0
	for _, row := range profile.Rows {
		if row.Threshold <= 0 || row.Threshold > 1_000_000 {
			return false
		}
		tierSizes[row.Threshold]++
		maxThreshold = max(maxThreshold, row.Threshold)
	}
	if maxThreshold != 1_000_000 {
		return false
	}
	for _, count := range tierSizes {
		if count < profile.DrawCount {
			return false
		}
	}
	return true
}

func chestProfileTiers(profile chestProfileSource) []chestTier {
	byThreshold := make(map[int][]chestContentSourceRow)
	thresholds := make([]int, 0, len(profile.Rows))
	seen := make(map[int]bool)
	for _, row := range profile.Rows {
		byThreshold[row.Threshold] = append(byThreshold[row.Threshold], row)
		if !seen[row.Threshold] {
			seen[row.Threshold] = true
			thresholds = append(thresholds, row.Threshold)
		}
	}
	sort.Ints(thresholds)
	result := make([]chestTier, 0, len(thresholds))
	previous := 0
	for _, threshold := range thresholds {
		chance := float64(threshold-previous) / 10_000.0
		if chance > 0 {
			result = append(result, chestTier{Threshold: threshold, Chance: chance, Rows: byThreshold[threshold]})
		}
		previous = threshold
	}
	return result
}

func chestTierItemChance(tier chestTier, drawCount, itemID int) float64 {
	n := len(tier.Rows)
	if n == 0 || drawCount <= 0 {
		return 0
	}
	k := min(drawCount, n)
	m := 0
	for _, row := range tier.Rows {
		if row.ItemID == itemID {
			m++
		}
	}
	if m == 0 {
		return 0
	}
	conditional := 1.0
	if n-m >= k {
		conditional -= combination(n-m, k) / combination(n, k)
	}
	return tier.Chance * conditional
}

func chestRowChance(tier chestTier, drawCount int) float64 {
	if len(tier.Rows) == 0 || drawCount <= 0 {
		return 0
	}
	return tier.Chance * float64(min(drawCount, len(tier.Rows))) / float64(len(tier.Rows))
}

func chestContentsForProfile(chestID int, profile chestProfileSource) *ChestContents {
	if len(profile.Rows) == 0 || profile.DrawCount <= 0 {
		return nil
	}
	name := store.itemNames[chestID]
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Сундук ID %d", chestID)
	}
	chanceKnown := chestProbabilityKnown(profile)
	byItem := make(map[int]*ChestContentItem)
	entryFor := func(itemID int) *ChestContentItem {
		entry := byItem[itemID]
		if entry != nil {
			return entry
		}
		itemName := store.itemNames[itemID]
		itemKnown := strings.TrimSpace(itemName) != ""
		if !itemKnown {
			itemName = fmt.Sprintf("Неизвестный предмет (ID %d)", itemID)
		}
		entry = &ChestContentItem{ItemID: itemID, Item: itemName, ItemKnown: itemKnown, ChanceKnown: chanceKnown}
		byItem[itemID] = entry
		return entry
	}

	if chanceKnown {
		for _, tier := range chestProfileTiers(profile) {
			rowChance := chestRowChance(tier, profile.DrawCount)
			seenInTier := make(map[int]bool)
			for _, row := range tier.Rows {
				entry := entryFor(row.ItemID)
				entry.Variants = append(entry.Variants, ChestVariant{Quantity: max(1, row.Quantity), Enhanced: max(0, row.Enhanced), Chance: rowChance, ChanceKnown: true})
				if !seenInTier[row.ItemID] {
					entry.Chance += chestTierItemChance(tier, profile.DrawCount, row.ItemID)
					seenInTier[row.ItemID] = true
				}
			}
		}
	} else {
		for _, row := range profile.Rows {
			entry := entryFor(row.ItemID)
			entry.Variants = append(entry.Variants, ChestVariant{Quantity: max(1, row.Quantity), Enhanced: max(0, row.Enhanced), ChanceKnown: false})
		}
	}

	items := make([]ChestContentItem, 0, len(byItem))
	for _, item := range byItem {
		sort.SliceStable(item.Variants, func(i, j int) bool {
			if item.Variants[i].ChanceKnown != item.Variants[j].ChanceKnown {
				return item.Variants[i].ChanceKnown
			}
			if item.Variants[i].ChanceKnown && math.Abs(item.Variants[i].Chance-item.Variants[j].Chance) > 0.0000001 {
				return item.Variants[i].Chance > item.Variants[j].Chance
			}
			if item.Variants[i].Quantity != item.Variants[j].Quantity {
				return item.Variants[i].Quantity > item.Variants[j].Quantity
			}
			return item.Variants[i].Enhanced > item.Variants[j].Enhanced
		})
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ChanceKnown != items[j].ChanceKnown {
			return items[i].ChanceKnown
		}
		if items[i].ChanceKnown && math.Abs(items[i].Chance-items[j].Chance) > 0.0000001 {
			return items[i].Chance > items[j].Chance
		}
		left := strings.ToLower(strings.TrimSpace(items[i].Item))
		right := strings.ToLower(strings.TrimSpace(items[j].Item))
		if left != right {
			return left < right
		}
		return items[i].ItemID < items[j].ItemID
	})
	return &ChestContents{ChestID: chestID, Chest: name, DrawCount: profile.DrawCount, Items: items}
}

func buildRuntime(server ServerData, chestProfiles map[int]chestProfileSource, monsterIDs map[int]struct{}) *runtimeData {
	rt := &runtimeData{
		server:           server,
		monsterIDs:       monsterIDs,
		resolved:         make(map[int][]DropItem, len(server.DropLists)),
		questByItem:      make(map[int][]ItemDrop),
		chestProfiles:    chestProfiles,
		chestByItem:      make(map[int][]ItemDrop),
		knownSourceItems: make(map[int]struct{}),
	}

	for key, entries := range server.DropLists {
		groupID, err := strconv.Atoi(key)
		if err != nil || groupID <= 0 {
			continue
		}
		items := make([]DropItem, 0, len(entries))
		for position, entry := range entries {
			name := store.itemNames[entry.ItemID]
			if name == "" {
				name = "Неизвестный предмет"
			}
			items = append(items, DropItem{
				ItemID:              entry.ItemID,
				Item:                name,
				Chance:              entry.Chance,
				BaseSelectionChance: orderedEntryChance(entries, position, func(value DropListEntry) float64 { return value.Chance }),
				Quantity:            max(1, entry.Quantity),
				Position:            position + 1,
			})
		}
		rt.resolved[groupID] = items
	}

	for _, drop := range store.data.QuestDrops {
		if drop.MonsterID > 0 && !monsterVisible(rt, drop.MonsterID) {
			continue
		}
		monsterLevel := 0
		if monster := store.monstersByID[drop.MonsterID]; monster != nil {
			monsterLevel = monster.Level
		}
		rt.knownSourceItems[drop.ItemID] = struct{}{}
		rt.questByItem[drop.ItemID] = append(rt.questByItem[drop.ItemID], ItemDrop{
			ItemID:           drop.ItemID,
			MonsterID:        drop.MonsterID,
			Monster:          drop.Monster,
			MonsterLevel:     monsterLevel,
			QuestID:          drop.QuestID,
			Quest:            drop.Quest,
			Source:           drop.Source,
			Context:          drop.Context,
			GroupTitle:       drop.GroupTitle,
			GroupChance:      drop.GroupChance,
			GroupChanceKnown: drop.GroupChanceKnown,
			ItemChance:       coalesceChance(drop.ItemChance, drop.Chance),
			ItemBaseChance:   coalesceChance(drop.ItemChance, drop.Chance),
			Quantity:         max(1, drop.Quantity),
			ItemPosition:     len(rt.questByItem[drop.ItemID]) + 1,
			SlotTitle:        "Условие задания",
		})
	}

	for chestID, profile := range chestProfiles {
		contents := chestContentsForProfile(chestID, profile)
		if contents == nil {
			continue
		}
		for index, content := range contents.Items {
			rt.knownSourceItems[content.ItemID] = struct{}{}
			quantity := 0
			if len(content.Variants) == 1 {
				quantity = content.Variants[0].Quantity
			}
			rt.chestByItem[content.ItemID] = append(rt.chestByItem[content.ItemID], ItemDrop{
				ItemID:         content.ItemID,
				ContainerID:    chestID,
				Container:      contents.Chest,
				Source:         "Сундук",
				Context:        "Открытие сундука",
				ItemChance:     content.Chance,
				ItemBaseChance: content.Chance,
				ChanceKnown:    content.ChanceKnown,
				Quantity:       quantity,
				ItemPosition:   index + 1,
				Variants:       append([]ChestVariant(nil), content.Variants...),
			})
		}
	}

	markGroupItems := func(groupID int) {
		for _, entry := range rt.resolved[groupID] {
			if entry.ItemID > 0 {
				rt.knownSourceItems[entry.ItemID] = struct{}{}
			}
		}
	}
	for index := range store.data.Monsters {
		monster := &store.data.Monsters[index]
		if !monsterVisible(rt, monster.ID) {
			continue
		}
		for _, slot := range directSlotsForMonster(server, monster.ID) {
			for _, choice := range slot.Choices {
				markGroupItems(choice.GroupID)
			}
		}
	}
	for _, rule := range server.WorldRules {
		for _, group := range rule.Groups {
			markGroupItems(group.GroupID)
		}
	}
	return rt
}

func directSlotsForMonster(server ServerData, monsterID int) []DropSlotRule {

	return server.DirectSlots[strconv.Itoa(monsterID)]
}

func worldMonsterTypeLabel(typeID int) string {
	switch typeID {
	case 0:
		return "любой тип монстра"
	case 13:
		return "только боссы"
	case 14:
		return "только рейдовые боссы"
	}
	if name := strings.TrimSpace(store.monsterTypeNames[typeID]); name != "" {
		return fmt.Sprintf("тип «%s»", name)
	}
	return fmt.Sprintf("тип монстра %d", typeID)
}

func worldRuleContext(rule WorldRule) string {
	context := map[int]string{1: "Открытая локация", 2: "Инстанс"}[rule.ContextID]
	if context == "" {
		context = fmt.Sprintf("Контекст %d", rule.ContextID)
	}
	return fmt.Sprintf("%s · уровни %d–%d · %s", context, rule.MinLevel, rule.MaxLevel, worldMonsterTypeLabel(rule.MonsterType))
}

func worldRuleSourceLabel(rule WorldRule) string {
	switch rule.MonsterType {
	case 0:
		return "Любой подходящий монстр"
	case 13:
		return "Босс"
	case 14:
		return "Рейдовый босс"
	default:
		return "Монстр, " + worldMonsterTypeLabel(rule.MonsterType)
	}
}

func worldRuleMatchesMonster(rule WorldRule, monster *Monster) bool {
	if monster == nil || monster.Level < rule.MinLevel || monster.Level > rule.MaxLevel {
		return false
	}
	return rule.MonsterType == 0 || rule.MonsterType == monster.Type
}

func monsterWorldRuleCount(monster *Monster, rt *runtimeData) int {
	if monster == nil || rt == nil {
		return 0
	}
	count := 0
	for _, rule := range rt.server.WorldRules {
		if worldRuleMatchesMonster(rule, monster) {
			count++
		}
	}
	return count
}

func monsterWorldDropView(monster *Monster, rt *runtimeData) []DropSlot {
	if monster == nil || rt == nil {
		return []DropSlot{}
	}
	slots := make([]DropSlot, 0, monsterWorldRuleCount(monster, rt))
	for _, rule := range rt.server.WorldRules {
		if !worldRuleMatchesMonster(rule, monster) {
			continue
		}
		chanceTotal := groupChanceTotal(rule.Groups)
		chanceOverflow := chanceTotal > 100.000001
		choices := make([]DropChoice, 0, len(rule.Groups))
		for choiceIndex, group := range rule.Groups {
			groupBaseChance := orderedEntryChance(rule.Groups, choiceIndex, func(value GroupRule) float64 { return value.Chance })
			items := dropItemsForAttempt(rt.resolved[group.GroupID], groupBaseChance)
			itemTotal, itemEmpty, itemOverflow := dropItemChanceSummary(items)
			choices = append(choices, DropChoice{
				ID:                  fmt.Sprintf("world-%d-%d", rule.SourceLine, choiceIndex+1),
				GroupID:             group.GroupID,
				Title:               fmt.Sprintf("Вариант %d", choiceIndex+1),
				Chance:              group.Chance,
				BaseSelectionChance: groupBaseChance,
				Items:               items,
				ItemChanceTotal:     itemTotal,
				ItemEmptyChance:     itemEmpty,
				ItemChanceOverflow:  itemOverflow,
			})
		}
		slots = append(slots, DropSlot{
			ID:               fmt.Sprintf("world-slot-%d", rule.SourceLine),
			Title:            "Мировая добыча",
			Source:           "Мировое выпадение",
			Context:          worldRuleContext(rule),
			SourceLine:       rule.SourceLine,
			AddAttempt1Count: rule.AddAttempt1Count,
			AddAttempt1Rate:  rule.AddAttempt1Rate,
			AddAttempt2Count: rule.AddAttempt2Count,
			AddAttempt2Rate:  rule.AddAttempt2Rate,
			ChanceTotal:      chanceTotal,
			EmptyChance:      remainingChance(chanceTotal, chanceOverflow),
			ChanceOverflow:   chanceOverflow,
			Choices:          choices,
			Note:             "Правило подходит этому монстру по уровню и типу. Для мировой добычи сервер отдельно учитывает тип локации; опубликованные данные не содержат надёжной связи каждого монстра с открытой локацией или инстансом.",
		})
	}
	return slots
}

func monsterDropView(monster *Monster, rt *runtimeData) ([]DropGroup, []DropSlot) {
	if monster == nil || rt == nil {
		return []DropGroup{}, []DropSlot{}
	}
	groups := make([]DropGroup, 0, 16)
	slots := make([]DropSlot, 0, 7)

	for slotIndex, slotRule := range directSlotsForMonster(rt.server, monster.ID) {
		slotNumber := slotIndex + 1
		chanceTotal := groupChanceTotal(slotRule.Choices)
		chanceOverflow := chanceTotal > 100.000001
		emptyChance := remainingChance(chanceTotal, chanceOverflow)
		choices := make([]DropChoice, 0, len(slotRule.Choices))
		for choiceIndex, rule := range slotRule.Choices {
			groupBaseChance := orderedEntryChance(slotRule.Choices, choiceIndex, func(value GroupRule) float64 { return value.Chance })
			items := dropItemsForAttempt(rt.resolved[rule.GroupID], groupBaseChance)
			itemTotal, itemEmpty, itemOverflow := dropItemChanceSummary(items)
			choiceTitle := fmt.Sprintf("Группа %d", rule.GroupID)
			choice := DropChoice{
				ID:                  fmt.Sprintf("direct-%d-%d-%d", slotNumber, choiceIndex+1, rule.GroupID),
				GroupID:             rule.GroupID,
				Title:               choiceTitle,
				Chance:              rule.Chance,
				BaseSelectionChance: groupBaseChance,
				Items:               items,
				ItemChanceTotal:     itemTotal,
				ItemEmptyChance:     itemEmpty,
				ItemChanceOverflow:  itemOverflow,
			}
			choices = append(choices, choice)
			groups = append(groups, DropGroup{
				ID:          choice.ID,
				Title:       fmt.Sprintf("Попытка выпадения №%d · %s", slotNumber, choiceTitle),
				Source:      "Выпадение монстра",
				Context:     "Обычная добыча",
				Chance:      rule.Chance,
				ChanceKnown: true,
				Items:       items,
				Note:        "Группа — один из взаимоисключающих вариантов этой попытки выпадения.",
			})
		}
		slots = append(slots, DropSlot{
			ID:               fmt.Sprintf("direct-slot-%d", slotNumber),
			Title:            fmt.Sprintf("Попытка выпадения №%d", slotNumber),
			Source:           "Выпадение монстра",
			Context:          "Обычная добыча",
			SlotNumber:       slotNumber,
			SourceLine:       slotRule.SourceLine,
			AddAttempt1Count: slotRule.AddAttempt1Count,
			AddAttempt1Rate:  slotRule.AddAttempt1Rate,
			AddAttempt2Count: slotRule.AddAttempt2Count,
			AddAttempt2Rate:  slotRule.AddAttempt2Rate,
			ChanceTotal:      chanceTotal,
			EmptyChance:      emptyChance,
			ChanceOverflow:   chanceOverflow,
			Choices:          choices,
			Note:             "Это отдельная попытка выпадения после победы над монстром. Выбирается не более одной группы, затем — не более одного предмета.",
		})
	}
	return groups, slots
}

func matchingGroupItems(rt *runtimeData, itemID int) (map[int][]DropItem, map[int]bool) {
	matches := make(map[int][]DropItem)
	overflow := make(map[int]bool)
	for groupID, items := range rt.resolved {
		for _, entry := range items {
			if entry.ItemID == itemID {
				matches[groupID] = append(matches[groupID], entry)
			}
		}
		if len(matches[groupID]) != 0 {
			_, _, overflow[groupID] = dropItemChanceSummary(items)
		}
	}
	return matches, overflow
}

func itemDropSources(itemID int, rt *runtimeData) []ItemDrop {
	if rt == nil {
		return []ItemDrop{}
	}
	drops := append([]ItemDrop(nil), rt.questByItem[itemID]...)
	drops = append(drops, rt.chestByItem[itemID]...)
	matches, itemOverflow := matchingGroupItems(rt, itemID)
	if len(matches) == 0 {
		sortItemDropSources(drops)
		return drops
	}

	for _, monster := range store.data.Monsters {
		if !monsterVisible(rt, monster.ID) {
			continue
		}
		for slotIndex, slotRule := range directSlotsForMonster(rt.server, monster.ID) {
			slotNumber := slotIndex + 1
			chanceTotal := groupChanceTotal(slotRule.Choices)
			chanceOverflow := chanceTotal > 100.000001
			for choiceIndex, rule := range slotRule.Choices {
				groupBaseChance := orderedEntryChance(slotRule.Choices, choiceIndex, func(value GroupRule) float64 { return value.Chance })
				entries := matches[rule.GroupID]
				for _, entry := range entries {
					drops = append(drops, ItemDrop{
						ItemID:            itemID,
						MonsterID:         monster.ID,
						Monster:           monster.Name,
						MonsterLevel:      monster.Level,
						Source:            "Выпадение монстра",
						Context:           "Обычная добыча",
						GroupTitle:        fmt.Sprintf("Группа %d", rule.GroupID),
						GroupID:           rule.GroupID,
						GroupChance:       rule.Chance,
						GroupChanceKnown:  true,
						GroupBaseChance:   groupBaseChance,
						ItemChance:        entry.Chance,
						ItemBaseChance:    entry.BaseSelectionChance,
						BaseAttemptChance: groupBaseChance * entry.BaseSelectionChance / 100,
						Quantity:          entry.Quantity,
						SlotNumber:        slotNumber,
						SlotTitle:         fmt.Sprintf("Попытка выпадения №%d", slotNumber),
						ChoicePosition:    choiceIndex + 1,
						ItemPosition:      entry.Position,
						SourceLine:        slotRule.SourceLine,
						ChanceOverflow:    chanceOverflow,
						ItemOverflow:      itemOverflow[rule.GroupID],
					})
				}
			}
		}
	}

	for worldIndex, worldRule := range rt.server.WorldRules {
		chanceTotal := groupChanceTotal(worldRule.Groups)
		chanceOverflow := chanceTotal > 100.000001
		for choiceIndex, rule := range worldRule.Groups {
			groupBaseChance := orderedEntryChance(worldRule.Groups, choiceIndex, func(value GroupRule) float64 { return value.Chance })
			entries := matches[rule.GroupID]
			for _, entry := range entries {
				drops = append(drops, ItemDrop{
					ItemID:            itemID,
					Monster:           worldRuleSourceLabel(worldRule),
					Source:            "Мировое выпадение",
					Context:           worldRuleContext(worldRule),
					GroupTitle:        fmt.Sprintf("Группа %d", rule.GroupID),
					GroupID:           rule.GroupID,
					GroupChance:       rule.Chance,
					GroupChanceKnown:  true,
					GroupBaseChance:   groupBaseChance,
					ItemChance:        entry.Chance,
					ItemBaseChance:    entry.BaseSelectionChance,
					BaseAttemptChance: groupBaseChance * entry.BaseSelectionChance / 100,
					Quantity:          entry.Quantity,
					SlotNumber:        worldIndex + 1,
					SlotTitle:         "Мировая добыча",
					ChoicePosition:    choiceIndex + 1,
					ItemPosition:      entry.Position,
					SourceLine:        worldRule.SourceLine,
					ChanceOverflow:    chanceOverflow,
					ItemOverflow:      itemOverflow[rule.GroupID],
				})
			}
		}
	}

	sortItemDropSources(drops)
	return drops
}

func itemDropSortChance(drop ItemDrop) float64 {
	if drop.GroupChanceKnown {
		return drop.BaseAttemptChance
	}
	return drop.ItemBaseChance
}

func itemDropSourceRank(source string) int {
	switch source {
	case "Выпадение монстра":
		return 0
	case "Мировое выпадение":
		return 1
	case "Сундук":
		return 2
	case "Квестовое выпадение", "Квестовый дроп":
		return 3
	default:
		return 4
	}
}

func sortItemDropSources(drops []ItemDrop) {
	sort.SliceStable(drops, func(i, j int) bool {
		leftRank := itemDropSourceRank(drops[i].Source)
		rightRank := itemDropSourceRank(drops[j].Source)
		if leftRank != rightRank {
			return leftRank < rightRank
		}

		leftChance := itemDropSortChance(drops[i])
		rightChance := itemDropSortChance(drops[j])
		if math.Abs(leftChance-rightChance) > 0.0000001 {
			return leftChance > rightChance
		}
		if drops[i].Source == "Выпадение монстра" && drops[i].MonsterLevel != drops[j].MonsterLevel {
			return drops[i].MonsterLevel < drops[j].MonsterLevel
		}

		leftName := strings.ToLower(strings.TrimSpace(sourceDropSortName(drops[i])))
		rightName := strings.ToLower(strings.TrimSpace(sourceDropSortName(drops[j])))
		if leftName != rightName {
			return leftName < rightName
		}
		if drops[i].MonsterID != drops[j].MonsterID {
			return drops[i].MonsterID < drops[j].MonsterID
		}
		if drops[i].ContainerID != drops[j].ContainerID {
			return drops[i].ContainerID < drops[j].ContainerID
		}
		if drops[i].SlotNumber != drops[j].SlotNumber {
			return drops[i].SlotNumber < drops[j].SlotNumber
		}
		if drops[i].ChoicePosition != drops[j].ChoicePosition {
			return drops[i].ChoicePosition < drops[j].ChoicePosition
		}
		return drops[i].ItemPosition < drops[j].ItemPosition
	})
}

func sourceDropSortName(drop ItemDrop) string {
	if strings.TrimSpace(drop.Container) != "" {
		return drop.Container
	}
	if strings.TrimSpace(drop.Quest) != "" {
		return drop.Quest
	}
	return drop.Monster
}

func groupChanceTotal(rules []GroupRule) float64 {
	total := 0.0
	for _, rule := range rules {
		total += rule.Chance
	}
	return total
}

func remainingChance(total float64, overflow bool) float64 {
	if !overflow && total < 100 {
		return 100 - total
	}
	return 0
}

func dropItemChanceSummary(items []DropItem) (total, empty float64, overflow bool) {
	for _, item := range items {
		total += item.Chance
	}
	overflow = total > 100.000001
	if !overflow && total < 100 {
		empty = 100 - total
	}
	return total, empty, overflow
}

func coalesceChance(a, b float64) float64 {
	if a != 0 {
		return a
	}
	return b
}

func orderedEffectiveChance(weights []float64, index int) float64 {
	if index < 0 || index >= len(weights) {
		return 0
	}
	before := 0.0
	for i := 0; i < index; i++ {
		before += math.Max(0, weights[i])
	}
	after := before + math.Max(0, weights[index])
	return math.Max(0, math.Min(100, after)-math.Min(100, before))
}

func orderedEntryChance[T any](entries []T, index int, weight func(T) float64) float64 {
	weights := make([]float64, len(entries))
	for i, entry := range entries {
		weights[i] = weight(entry)
	}
	return orderedEffectiveChance(weights, index)
}

func dropItemsForAttempt(items []DropItem, groupBaseChance float64) []DropItem {
	result := append([]DropItem(nil), items...)
	for i := range result {
		result[i].BaseAttemptChance = groupBaseChance * result[i].BaseSelectionChance / 100
	}
	return result
}

func chestContents(chestID int, rt *runtimeData) *ChestContents {
	if rt == nil {
		return nil
	}
	profile, ok := rt.chestProfiles[chestID]
	if !ok {
		return nil
	}
	return chestContentsForProfile(chestID, profile)
}

func worldSourceMonsters(itemID, sourceLine, groupID, choicePosition, itemPosition int, rt *runtimeData) ([]WorldSourceMonster, string, bool) {
	if rt == nil || itemID <= 0 || sourceLine <= 0 || groupID <= 0 || choicePosition <= 0 || itemPosition <= 0 {
		return nil, "", false
	}
	var selected *WorldRule
	for index := range rt.server.WorldRules {
		rule := &rt.server.WorldRules[index]
		if rule.SourceLine == sourceLine {
			selected = rule
			break
		}
	}
	if selected == nil {
		return nil, "", false
	}
	groupIndex := choicePosition - 1
	if groupIndex < 0 || groupIndex >= len(selected.Groups) || selected.Groups[groupIndex].GroupID != groupID {
		return nil, "", false
	}
	entries := rt.resolved[groupID]
	var entry *DropItem
	for index := range entries {
		if entries[index].ItemID == itemID && entries[index].Position == itemPosition {
			entry = &entries[index]
			break
		}
	}
	if entry == nil {
		return nil, "", false
	}
	groupBase := orderedEntryChance(selected.Groups, groupIndex, func(value GroupRule) float64 { return value.Chance })
	chance := groupBase * entry.BaseSelectionChance / 100
	monsters := make([]WorldSourceMonster, 0, 64)
	for index := range store.data.Monsters {
		monster := &store.data.Monsters[index]
		if !monsterVisible(rt, monster.ID) || !worldRuleMatchesMonster(*selected, monster) {
			continue
		}
		monsters = append(monsters, WorldSourceMonster{MonsterID: monster.ID, Monster: monster.Name, Level: monster.Level, Chance: chance})
	}
	sort.SliceStable(monsters, func(i, j int) bool {
		if math.Abs(monsters[i].Chance-monsters[j].Chance) > 0.0000001 {
			return monsters[i].Chance > monsters[j].Chance
		}
		if monsters[i].Level != monsters[j].Level {
			return monsters[i].Level < monsters[j].Level
		}
		left := strings.ToLower(strings.TrimSpace(monsters[i].Monster))
		right := strings.ToLower(strings.TrimSpace(monsters[j].Monster))
		if left != right {
			return left < right
		}
		return monsters[i].MonsterID < monsters[j].MonsterID
	})
	return monsters, worldRuleContext(*selected), true
}

func activeRuntime(server string) *runtimeData {
	if server == "or" {
		server = "original"
	}
	slot := store.runtimes[server]
	if slot == nil {
		slot = store.runtimes["kiss"]
	}
	if slot == nil {
		for _, candidate := range store.runtimes {
			slot = candidate
			break
		}
	}
	if slot == nil {
		return &runtimeData{monsterIDs: map[int]struct{}{}, resolved: map[int][]DropItem{}, questByItem: map[int][]ItemDrop{}, chestByItem: map[int][]ItemDrop{}, knownSourceItems: map[int]struct{}{}}
	}
	slot.once.Do(func() { slot.value = buildRuntime(slot.server, slot.chestProfiles, slot.monsterIDs) })
	return slot.value
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func parseInt(q url.Values, key string, fallback int) int {
	n, err := strconv.Atoi(q.Get(key))
	if err != nil {
		return fallback
	}
	return n
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func limitedQueryValue(values url.Values, key string, maxRunes int) (string, bool) {
	value := strings.TrimSpace(values.Get(key))
	if len([]rune(value)) > maxRunes {
		return "", false
	}
	return value, true
}

func itemLevel(item *Item) int {
	if item.MinLevel > 1 {
		return item.MinLevel
	}
	if item.MaxLevel > 0 && item.MaxLevel < 100 {
		return item.MaxLevel
	}
	return 0
}

func recipeMasteryLevel(item *Item) int {
	if item == nil || item.MakeSkill <= 0 {
		return 0
	}
	return max(0, item.MakeSkillExp)
}

func normalizeSearch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "ё", "е")
	var builder strings.Builder
	builder.Grow(len(value))
	spacePending := false
	for _, r := range value {
		if (r >= 'а' && r <= 'я') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if spacePending && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			spacePending = false
			builder.WriteRune(r)
		} else {
			spacePending = true
		}
	}
	return strings.TrimSpace(builder.String())
}

var russianSearchSuffixes = []string{
	"иями", "ями", "ами", "ией", "иям", "ием", "иях", "ью", "ия", "ья",
	"ого", "ему", "ому", "ыми", "ими", "его", "ее", "ие", "ые", "ое",
	"ей", "ий", "ый", "ой", "ем", "им", "ым", "ом", "их", "ых",
	"ую", "юю", "ая", "яя", "ев", "ов", "ам", "ям", "ах", "ях",
	"а", "я", "ы", "и", "ь", "й", "у", "ю", "о", "е",
}

func russianSearchStem(word string) string {
	runes := []rune(word)
	if len(runes) <= 3 {
		return word
	}
	for _, suffix := range russianSearchSuffixes {
		suffixRunes := []rune(suffix)
		if len(runes)-len(suffixRunes) < 3 || !strings.HasSuffix(word, suffix) {
			continue
		}
		return string(runes[:len(runes)-len(suffixRunes)])
	}
	return word
}

func stemSearch(value string) string {
	normalized := normalizeSearch(value)
	if normalized == "" {
		return ""
	}
	words := strings.Fields(normalized)
	for i, word := range words {
		words[i] = russianSearchStem(word)
	}
	return strings.Join(words, " ")
}

func newSearchDocument(value string) searchDocument {
	literal := normalizeSearch(value)
	return searchDocument{Literal: literal, Stems: stemSearch(literal)}
}

func matchesSearch(document searchDocument, query string) bool {
	literal := normalizeSearch(query)
	if literal == "" {
		return true
	}
	if strings.Contains(document.Literal, literal) {
		return true
	}
	stems := stemSearch(literal)
	if stems == "" {
		return false
	}
	if strings.Contains(document.Stems, stems) {
		return true
	}
	documentWords := strings.Fields(document.Stems)
	for _, queryWord := range strings.Fields(stems) {
		found := false
		for _, documentWord := range documentWords {
			if documentWord == queryWord {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func handleMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	categories := make([]map[string]any, 0, len(store.categoryItems))
	for name, count := range store.categoryItems {
		categories = append(categories, map[string]any{"name": name, "count": count})
	}
	sort.Slice(categories, func(i, j int) bool { return categories[i]["count"].(int) > categories[j]["count"].(int) })
	if len(categories) > 8 {
		categories = categories[:8]
	}
	servers := make([]map[string]any, 0, len(store.data.Servers))
	for key, s := range store.data.Servers {
		servers = append(servers, map[string]any{"key": key, "name": s.Name, "groups": s.DropListGroups, "slots": s.DirectDropSlots, "directDropsUpdatedAt": s.DirectDropsUpdatedAt, "dropListsUpdatedAt": s.DropListsUpdatedAt, "worldDropsUpdatedAt": s.WorldDropsUpdatedAt})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i]["key"].(string) < servers[j]["key"].(string) })
	writeJSON(w, map[string]any{"meta": store.data.Meta, "categories": categories, "servers": servers, "effectSpecs": store.data.EffectSpecs})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	qv := r.URL.Query()
	q, ok := limitedQueryValue(qv, "q", 160)
	if !ok {
		http.Error(w, "Слишком длинный поисковый запрос.\n", http.StatusBadRequest)
		return
	}
	if q == "" {
		writeJSON(w, map[string]any{"items": []any{}, "monsters": []any{}})
		return
	}
	query := q
	rt := activeRuntime(qv.Get("server"))
	items := make([]map[string]any, 0, 6)
	for i := range store.data.Items {
		if matchesSearch(store.itemSearch[i], query) {
			items = append(items, itemSummary(&store.data.Items[i]))
			if len(items) == 6 {
				break
			}
		}
	}
	monsters := make([]map[string]any, 0, 4)
	for i := range store.data.Monsters {
		if !monsterVisible(rt, store.data.Monsters[i].ID) {
			continue
		}
		if matchesSearch(store.monsterSearch[i], query) {
			monsters = append(monsters, monsterSummary(&store.data.Monsters[i]))
			if len(monsters) == 4 {
				break
			}
		}
	}
	writeJSON(w, map[string]any{"items": items, "monsters": monsters})
}

func handleItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/api/items" {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	qv := r.URL.Query()
	query, queryOK := limitedQueryValue(qv, "q", 160)
	category, categoryOK := limitedQueryValue(qv, "category", 120)
	subcategory, subcategoryOK := limitedQueryValue(qv, "subcategory", 120)
	quality, qualityOK := limitedQueryValue(qv, "quality", 120)
	if !queryOK || !categoryOK || !subcategoryOK || !qualityOK {
		http.Error(w, "Слишком длинное значение фильтра.\n", http.StatusBadRequest)
		return
	}
	queryNormalized := query
	if category == "" {
		subcategory = ""
		quality = ""
	}
	scope := qv.Get("scope")
	knownSource := qv.Get("knownSource")
	if knownSource != "" && knownSource != "1" {
		http.Error(w, "Некорректное значение фильтра источника.\n", http.StatusBadRequest)
		return
	}
	var rt *runtimeData
	if knownSource == "1" {
		rt = activeRuntime(qv.Get("server"))
	}
	minLevel := clampInt(parseInt(qv, "minLevel", 0), 0, 10000)
	maxLevel := clampInt(parseInt(qv, "maxLevel", 0), 0, 10000)
	sortMode, sortOK := limitedQueryValue(qv, "sort", 20)
	if !sortOK || (sortMode != "" && sortMode != "name" && sortMode != "level" && sortMode != "rarity") {
		http.Error(w, "Некорректный режим сортировки.\n", http.StatusBadRequest)
		return
	}
	page := clampInt(parseInt(qv, "page", 1), 1, 100000)
	pageSize := clampInt(parseInt(qv, "pageSize", 20), 8, 48)

	filtered := make([]*Item, 0, 256)
	categories := map[string]bool{}
	subcategories := map[string]bool{}
	qualities := map[string]int{}
	for i := range store.data.Items {
		item := &store.data.Items[i]
		if scope == "weapons" && item.Category != "Оружие/щит" {
			continue
		}
		if scope == "armor" && !strings.HasPrefix(item.Category, "Броня") {
			continue
		}
		if item.Category != "Доп. умения" {
			categories[item.Category] = true
		}
		if category != "" && item.Category != category {
			continue
		}
		subcategories[item.Subcategory] = true
		if qualityName := strings.TrimSpace(item.Quality); qualityName != "" {
			if qualityID, ok := qualities[qualityName]; !ok || item.QualityID < qualityID {
				qualities[qualityName] = item.QualityID
			}
		}
		if subcategory != "" && item.Subcategory != subcategory {
			continue
		}
		if quality != "" && item.Quality != quality {
			continue
		}
		level := itemLevel(item)
		if minLevel > 0 && level < minLevel {
			continue
		}
		if maxLevel > 0 && level > maxLevel {
			continue
		}
		if knownSource == "1" {
			if _, ok := rt.knownSourceItems[item.ID]; !ok {
				continue
			}
		}
		if queryNormalized != "" && !matchesSearch(store.itemSearch[i], queryNormalized) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		leftNameClass := catalogNameClass(filtered[i].Name)
		rightNameClass := catalogNameClass(filtered[j].Name)
		if leftNameClass == 4 || rightNameClass == 4 {
			if leftNameClass != rightNameClass {
				return leftNameClass < rightNameClass
			}
		}
		if sortMode == "level" {
			li, lj := itemLevel(filtered[i]), itemLevel(filtered[j])
			if li != lj {
				return li < lj
			}
		}
		if sortMode == "rarity" && filtered[i].QualityID != filtered[j].QualityID {
			return filtered[i].QualityID < filtered[j].QualityID
		}
		return catalogNameLess(filtered[i].Name, filtered[j].Name)
	})
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := min(total, start+pageSize)
	result := make([]map[string]any, 0, end-start)
	for _, item := range filtered[start:end] {
		result = append(result, itemSummary(item))
	}
	if category == "" {
		subcategories = map[string]bool{}
		qualities = map[string]int{}
	}
	writeJSON(w, map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize, "pages": max(1, (total+pageSize-1)/pageSize), "filters": map[string]any{"categories": sortedItemCategories(categories), "subcategories": sortedKeys(subcategories), "qualities": sortedQualityKeys(qualities)}})
}

func itemSetMemberPresentationOrder(member ItemSetMember) int {
	item := store.itemsByID[member.ItemID]
	if item == nil {
		return 10_000
	}

	if item.MainCategoryID == 2 && item.MiddleCategoryID >= 201 && item.MiddleCategoryID <= 215 {
		slot := (item.MiddleCategoryID - 201) % 5
		return [...]int{0, 1, 3, 2, 4}[slot]
	}
	return 10_000
}

func itemSetForPresentation(value ItemSet) ItemSet {
	value.Items = append([]ItemSetMember(nil), value.Items...)
	sort.SliceStable(value.Items, func(i, j int) bool {
		return itemSetMemberPresentationOrder(value.Items[i]) < itemSetMemberPresentationOrder(value.Items[j])
	})
	return value
}

func recipeSourceTypeLabel(source string) string {
	switch source {
	case "Выпадение монстра":
		return "Монстр"
	case "Мировое выпадение":
		return "Мировая добыча"
	case "Сундук":
		return "Сундук"
	case "Квестовое выпадение", "Квестовый дроп":
		return "Задание"
	default:
		if strings.TrimSpace(source) != "" {
			return source
		}
		return "Источник"
	}
}

func recipeSourceName(drop ItemDrop) string {
	switch drop.Source {
	case "Мировое выпадение":
		if strings.TrimSpace(drop.Monster) != "" {
			return drop.Monster
		}
		return "Мировая добыча"
	case "Сундук":
		if strings.TrimSpace(drop.Container) != "" {
			return drop.Container
		}
		return "Сундук"
	case "Квестовое выпадение", "Квестовый дроп":
		if strings.TrimSpace(drop.Quest) != "" {
			return drop.Quest
		}
		return "Задание"
	default:
		if strings.TrimSpace(drop.Monster) != "" {
			return drop.Monster
		}
		return "Источник"
	}
}

func recipeSummary(item *Item, rt *runtimeData) map[string]any {
	result := itemSummary(item)
	materials := itemRecipeMaterials(item.ID)
	result["materials"] = materials
	result["materialKinds"] = len(materials)
	totalQuantity := 0
	for _, material := range materials {
		totalQuantity += max(1, material.Quantity)
	}
	result["materialQuantity"] = totalQuantity
	result["masteryLevel"] = recipeMasteryLevel(item)
	result["makeSkill"] = item.MakeSkill
	drops := itemDropSources(item.ID, rt)
	result["sourceCount"] = len(drops)
	if len(drops) > 0 {
		result["sourcePreview"] = map[string]any{
			"type": recipeSourceTypeLabel(drops[0].Source),
			"name": recipeSourceName(drops[0]),
		}
	}
	return result
}

func handleRecipes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/api/recipes" {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	qv := r.URL.Query()
	query, queryOK := limitedQueryValue(qv, "q", 160)
	recipeType, typeOK := limitedQueryValue(qv, "type", 120)
	quality, qualityOK := limitedQueryValue(qv, "quality", 120)
	if !queryOK || !typeOK || !qualityOK {
		http.Error(w, "Слишком длинное значение фильтра.\n", http.StatusBadRequest)
		return
	}
	knownSource := qv.Get("knownSource")
	if knownSource != "" && knownSource != "1" {
		http.Error(w, "Некорректное значение фильтра источника.\n", http.StatusBadRequest)
		return
	}
	rt := activeRuntime(qv.Get("server"))
	minLevel := clampInt(parseInt(qv, "minLevel", 0), 0, 10000)
	maxLevel := clampInt(parseInt(qv, "maxLevel", 0), 0, 10000)
	sortMode, sortOK := limitedQueryValue(qv, "sort", 20)
	if !sortOK || (sortMode != "" && sortMode != "name" && sortMode != "level" && sortMode != "mastery" && sortMode != "rarity") {
		http.Error(w, "Некорректный режим сортировки.\n", http.StatusBadRequest)
		return
	}
	page := clampInt(parseInt(qv, "page", 1), 1, 100000)
	pageSize := clampInt(parseInt(qv, "pageSize", 20), 8, 48)

	filtered := make([]*Item, 0, len(store.itemRecipes))
	types := map[string]bool{}
	qualities := map[string]int{}
	for id, sourceMaterials := range store.itemRecipes {
		item := store.itemsByID[id]
		if item == nil {
			continue
		}
		if strings.TrimSpace(item.Subcategory) != "" {
			types[item.Subcategory] = true
		}
		if qualityName := strings.TrimSpace(item.Quality); qualityName != "" {
			if qualityID, ok := qualities[qualityName]; !ok || item.QualityID < qualityID {
				qualities[qualityName] = item.QualityID
			}
		}
		if recipeType != "" && item.Subcategory != recipeType {
			continue
		}
		if quality != "" && item.Quality != quality {
			continue
		}
		masteryLevel := recipeMasteryLevel(item)
		if minLevel > 0 && masteryLevel < minLevel {
			continue
		}
		if maxLevel > 0 && masteryLevel > maxLevel {
			continue
		}
		if knownSource == "1" {
			if _, ok := rt.knownSourceItems[item.ID]; !ok {
				continue
			}
		}
		if query != "" {
			var search strings.Builder
			fmt.Fprintf(&search, "%d %s %s %s %s", item.ID, item.Name, item.TypeLine, item.Subcategory, item.Quality)
			for _, material := range sourceMaterials {
				if materialItem := store.itemsByID[material.ItemID]; materialItem != nil {
					fmt.Fprintf(&search, " %d %s", material.ItemID, materialItem.Name)
				} else {
					fmt.Fprintf(&search, " %d", material.ItemID)
				}
			}
			if !matchesSearch(newSearchDocument(search.String()), query) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		leftNameClass := catalogNameClass(filtered[i].Name)
		rightNameClass := catalogNameClass(filtered[j].Name)
		if leftNameClass == 4 || rightNameClass == 4 {
			if leftNameClass != rightNameClass {
				return leftNameClass < rightNameClass
			}
		}
		if sortMode == "level" || sortMode == "mastery" {
			li, lj := recipeMasteryLevel(filtered[i]), recipeMasteryLevel(filtered[j])
			if li != lj {
				return li < lj
			}
		}
		if sortMode == "rarity" && filtered[i].QualityID != filtered[j].QualityID {
			return filtered[i].QualityID < filtered[j].QualityID
		}
		return catalogNameLess(filtered[i].Name, filtered[j].Name)
	})
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := min(total, start+pageSize)
	result := make([]map[string]any, 0, end-start)
	for _, item := range filtered[start:end] {
		result = append(result, recipeSummary(item, rt))
	}
	writeJSON(w, map[string]any{
		"recipes":  result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"pages":    max(1, (total+pageSize-1)/pageSize),
		"filters":  map[string]any{"types": sortedKeys(types), "qualities": sortedQualityKeys(qualities)},
	})
}

func itemBonuses(item *Item) []map[string]any {
	bonuses := make([]map[string]any, 0, len(item.Options))
	for _, option := range item.Options {
		spec, ok := store.data.EffectSpecs[strconv.Itoa(option.Type)]
		name := spec.Name
		if !ok || name == "" {
			name = fmt.Sprintf("Неизвестный эффект (код %d)", option.Type)
		}
		value := fmt.Sprintf("%+d", option.Value)
		if spec.Percent {
			value = fmt.Sprintf("%+.2f%%", float64(option.Value))
		}
		bonuses = append(bonuses, map[string]any{"type": option.Type, "name": name, "value": value, "known": ok && strings.TrimSpace(spec.Name) != ""})
	}
	return bonuses
}

func recipeProductNameKey(name string) string {
	value := strings.TrimSpace(name)
	lower := strings.ToLower(value)
	const recipePrefix = "[рецепт]"
	if strings.HasPrefix(lower, recipePrefix) {
		value = strings.TrimSpace(value[len(recipePrefix):])
	}

	idx := 0
	for idx < len(value) && value[idx] >= '0' && value[idx] <= '9' {
		idx++
	}
	if idx > 0 {
		rest := strings.TrimSpace(value[idx:])
		lowerRest := strings.ToLower(rest)
		if strings.HasPrefix(lowerRest, "ур.") {
			value = strings.TrimSpace(rest[len("ур."):])
		} else if strings.HasPrefix(lowerRest, "ур ") {
			value = strings.TrimSpace(rest[len("ур "):])
		}
	}
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func recipeProduct(item *Item) *Item {
	if item == nil {
		return nil
	}
	key := recipeProductNameKey(item.Name)
	if key == "" {
		return nil
	}
	var match *Item
	for i := range store.data.Items {
		candidate := &store.data.Items[i]
		if candidate.ID == item.ID {
			continue
		}
		if _, isRecipe := store.itemRecipes[candidate.ID]; isRecipe {
			continue
		}
		if recipeProductNameKey(candidate.Name) != key {
			continue
		}
		if match != nil {
			return nil
		}
		match = candidate
	}
	return match
}

func handleItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/items/"))
	if err != nil {
		http.Error(w, "Некорректный идентификатор предмета.\n", http.StatusBadRequest)
		return
	}
	item := store.itemsByID[id]
	if item == nil {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	server := r.URL.Query().Get("server")
	rt := activeRuntime(server)
	var set *ItemSet
	if item.SetIndex > 0 {
		value, ok := store.data.ItemSets[strconv.Itoa(item.SetIndex)]
		if ok {
			value = itemSetForPresentation(value)
			set = &value
		}
	}
	bonuses := itemBonuses(item)
	var product map[string]any
	if _, isRecipe := store.itemRecipes[id]; isRecipe {
		if crafted := recipeProduct(item); crafted != nil {
			product = map[string]any{
				"item":    crafted,
				"level":   itemLevel(crafted),
				"bonuses": itemBonuses(crafted),
			}
		}
	}
	writeJSON(w, map[string]any{"item": item, "level": itemLevel(item), "bonuses": bonuses, "set": set, "recipe": itemRecipeMaterials(id), "recipeProduct": product, "chest": chestContents(id, rt), "drops": itemDropSources(id, rt)})
}

func handleWorldSourceMonsters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/api/world-source-monsters" {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	itemID := parseInt(q, "itemId", 0)
	sourceLine := parseInt(q, "sourceLine", 0)
	groupID := parseInt(q, "groupId", 0)
	choicePosition := parseInt(q, "choicePosition", 0)
	itemPosition := parseInt(q, "itemPosition", 0)
	if itemID <= 0 || sourceLine <= 0 || groupID <= 0 || choicePosition <= 0 || itemPosition <= 0 {
		http.Error(w, "Некорректные параметры источника.\n", http.StatusBadRequest)
		return
	}
	monsters, context, ok := worldSourceMonsters(itemID, sourceLine, groupID, choicePosition, itemPosition, activeRuntime(q.Get("server")))
	if !ok {
		http.Error(w, "Источник не найден.\n", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{
		"monsters":          monsters,
		"total":             len(monsters),
		"context":           context,
		"contextMatchKnown": false,
	})
}

func handleMonsterWorldDrops(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/api/monster-world-drops" {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	monsterID := parseInt(q, "monsterId", 0)
	if monsterID <= 0 {
		http.Error(w, "Некорректный идентификатор монстра.\n", http.StatusBadRequest)
		return
	}
	rt := activeRuntime(q.Get("server"))
	monster := store.monstersByID[monsterID]
	if monster == nil || !monsterVisible(rt, monsterID) {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	slots := monsterWorldDropView(monster, rt)
	writeJSON(w, map[string]any{
		"monsterId":         monsterID,
		"slots":             slots,
		"contextMatchKnown": false,
	})
}

func handleMonsters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Path != "/api/monsters" {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	qv := r.URL.Query()
	rt := activeRuntime(qv.Get("server"))
	query, queryOK := limitedQueryValue(qv, "q", 160)
	category, categoryOK := limitedQueryValue(qv, "category", 120)
	typeName, typeOK := limitedQueryValue(qv, "type", 120)
	if !queryOK || !categoryOK || !typeOK {
		http.Error(w, "Слишком длинное значение фильтра.\n", http.StatusBadRequest)
		return
	}
	queryNormalized := query
	if category == "" {
		typeName = ""
	}
	minLevel := clampInt(parseInt(qv, "minLevel", 0), 0, 10000)
	maxLevel := clampInt(parseInt(qv, "maxLevel", 0), 0, 10000)
	sortMode, sortOK := limitedQueryValue(qv, "sort", 20)
	if !sortOK || (sortMode != "" && sortMode != "name" && sortMode != "level") {
		http.Error(w, "Некорректный режим сортировки.\n", http.StatusBadRequest)
		return
	}
	page := clampInt(parseInt(qv, "page", 1), 1, 100000)
	pageSize := clampInt(parseInt(qv, "pageSize", 20), 8, 48)
	categories := map[string]bool{}
	types := map[string]bool{}
	filtered := make([]*Monster, 0, 256)
	for i := range store.data.Monsters {
		mon := &store.data.Monsters[i]
		if !monsterVisible(rt, mon.ID) {
			continue
		}
		categories[mon.Category] = true
		types[mon.TypeName] = true
		if category != "" && mon.Category != category {
			continue
		}
		if typeName != "" && mon.TypeName != typeName {
			continue
		}
		if minLevel > 0 && mon.Level < minLevel {
			continue
		}
		if maxLevel > 0 && mon.Level > maxLevel {
			continue
		}
		if queryNormalized != "" && !matchesSearch(store.monsterSearch[i], queryNormalized) {
			continue
		}
		filtered = append(filtered, mon)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		leftNameClass := catalogNameClass(filtered[i].Name)
		rightNameClass := catalogNameClass(filtered[j].Name)
		if leftNameClass == 4 || rightNameClass == 4 {
			if leftNameClass != rightNameClass {
				return leftNameClass < rightNameClass
			}
		}
		if sortMode == "level" && filtered[i].Level != filtered[j].Level {
			return filtered[i].Level < filtered[j].Level
		}
		return catalogNameLess(filtered[i].Name, filtered[j].Name)
	})
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := min(total, start+pageSize)
	result := make([]map[string]any, 0, end-start)
	for _, mon := range filtered[start:end] {
		result = append(result, monsterSummary(mon))
	}
	if category == "" {
		types = map[string]bool{}
	}
	writeJSON(w, map[string]any{"monsters": result, "total": total, "page": page, "pageSize": pageSize, "pages": max(1, (total+pageSize-1)/pageSize), "filters": map[string]any{"categories": sortedKeys(categories), "types": sortedKeys(types)}})
}

func handleMonster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/monsters/"))
	if err != nil {
		http.Error(w, "Некорректный идентификатор монстра.\n", http.StatusBadRequest)
		return
	}
	rt := activeRuntime(r.URL.Query().Get("server"))
	mon := store.monstersByID[id]
	if mon == nil || !monsterVisible(rt, id) {
		http.Error(w, "Запись не найдена.\n", http.StatusNotFound)
		return
	}
	groups, slots := monsterDropView(mon, rt)
	writeJSON(w, map[string]any{"monster": mon, "groups": groups, "slots": slots, "worldRuleCount": monsterWorldRuleCount(mon, rt)})
}

func itemSummary(item *Item) map[string]any {
	setSize := 0
	if item.SetIndex > 0 {
		if set, ok := store.data.ItemSets[strconv.Itoa(item.SetIndex)]; ok {
			setSize = len(set.Items)
		}
	}
	return map[string]any{"id": item.ID, "name": item.Name, "typeLine": item.TypeLine, "category": item.Category, "subcategory": item.Subcategory, "quality": item.Quality, "qualityId": item.QualityID, "level": itemLevel(item), "description": strings.TrimSpace(item.Tooltip), "stats": itemStats(item), "setSize": setSize}
}

func monsterSummary(mon *Monster) map[string]any {
	stats := make([]map[string]any, 0, 4)
	add := func(name string, value any) {
		if number, ok := value.(int); ok && number == 0 {
			return
		}
		stats = append(stats, map[string]any{"name": name, "value": value})
	}
	add("HP", mon.HP)
	add("Физическая защита", mon.Defense)
	add("Магическая защита", mon.MagicDefense)
	return map[string]any{"id": mon.ID, "name": mon.Name, "category": mon.Category, "typeName": mon.TypeName, "level": mon.Level, "aggressive": mon.Aggressive, "stats": stats}
}

func itemStats(item *Item) []map[string]any {
	stats := make([]map[string]any, 0, 4)
	add := func(name string, value any) {
		if len(stats) < 4 {
			stats = append(stats, map[string]any{"name": name, "value": value})
		}
	}

	if item.PhysicalMin != 0 || item.PhysicalMax != 0 {
		add("Физическая атака", fmt.Sprintf("%d–%d", item.PhysicalMin, item.PhysicalMax))
	}
	if item.MagicMin != 0 || item.MagicMax != 0 {
		add("Магическая атака", fmt.Sprintf("%d–%d", item.MagicMin, item.MagicMax))
	}
	if item.Heal != 0 {
		add("Лечение", item.Heal)
	}
	if item.PhysicalDefense != 0 {
		add("Физическая защита", item.PhysicalDefense)
	}
	if item.MagicDefense != 0 {
		add("Магическая защита", item.MagicDefense)
	}
	if len(item.CardSlots) != 0 {
		add("Слоты карт", len(item.CardSlots))
	}
	if len(item.Options) != 0 {
		add("Эффекты", len(item.Options))
	}
	if item.AttackRange != 0 {
		add("Дальность", item.AttackRange)
	}
	if item.Price != 0 {
		add("Цена продажи", item.Price)
	}
	return stats
}

func isRussianCatalogLetter(r rune) bool {
	r = unicode.ToLower(r)
	return (r >= 'а' && r <= 'я') || r == 'ё'
}

func russianCatalogLetterOrder(r rune) int {
	r = unicode.ToLower(r)
	const alphabet = "абвгдеёжзийклмнопрстуфхцчшщъыьэюя"
	for index, letter := range []rune(alphabet) {
		if r == letter {
			return index
		}
	}
	return -1
}

func catalogNameClass(name string) int {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 4
	}
	first, _ := utf8.DecodeRuneInString(trimmed)
	if isRussianCatalogLetter(first) {
		return 0
	}
	if unicode.IsLetter(first) {
		return 1
	}
	if unicode.IsDigit(first) {
		return 2
	}
	return 3
}

func catalogFoldedTextLess(left, right string) bool {
	leftRunes := []rune(strings.ToLower(left))
	rightRunes := []rune(strings.ToLower(right))
	limit := min(len(leftRunes), len(rightRunes))
	for i := 0; i < limit; i++ {
		if leftRunes[i] == rightRunes[i] {
			continue
		}
		leftRussian := russianCatalogLetterOrder(leftRunes[i])
		rightRussian := russianCatalogLetterOrder(rightRunes[i])
		if leftRussian >= 0 && rightRussian >= 0 {
			return leftRussian < rightRussian
		}
		return leftRunes[i] < rightRunes[i]
	}
	return len(leftRunes) < len(rightRunes)
}

func catalogNameLess(left, right string) bool {
	leftTrimmed := strings.TrimSpace(left)
	rightTrimmed := strings.TrimSpace(right)
	leftClass := catalogNameClass(leftTrimmed)
	rightClass := catalogNameClass(rightTrimmed)
	if leftClass != rightClass {
		return leftClass < rightClass
	}
	if leftClass == 4 {
		return false
	}
	if strings.EqualFold(leftTrimmed, rightTrimmed) {
		return leftTrimmed < rightTrimmed
	}
	return catalogFoldedTextLess(leftTrimmed, rightTrimmed)
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			out = append(out, key)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return catalogNameLess(out[i], out[j])
	})
	return out
}

func itemCategoryFilterRank(category string) int {
	category = strings.TrimSpace(category)
	switch {
	case category == "Оружие/щит":
		return 0
	case strings.HasPrefix(category, "Броня"):
		return 1
	case category == "Бижутерия":
		return 2
	default:
		return 3
	}
}

func sortedItemCategories(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			out = append(out, key)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftRank := itemCategoryFilterRank(out[i])
		rightRank := itemCategoryFilterRank(out[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return catalogNameLess(out[i], out[j])
	})
	return out
}

func sortedQualityKeys(values map[string]int) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			out = append(out, key)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftID := values[out[i]]
		rightID := values[out[j]]
		if leftID != rightID {
			return leftID < rightID
		}
		return catalogNameLess(out[i], out[j])
	})
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func handleFavorites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		Keys     []string `json:"keys"`
		Server   string   `json:"server"`
		Page     int      `json:"page,omitempty"`
		PageSize int      `json:"pageSize,omitempty"`
	}
	if !decodeJSONRequest(w, r, &request, 1<<20) {
		return
	}
	rt := activeRuntime(request.Server)
	legacyRequest := request.Page == 0 && request.PageSize == 0
	maximumKeys := 5000
	if legacyRequest {
		maximumKeys = 500
	}
	if len(request.Keys) > maximumKeys {
		http.Error(w, "Слишком много записей в избранном.\n", http.StatusBadRequest)
		return
	}
	type favoriteReference struct {
		kind string
		id   int
	}
	references := make([]favoriteReference, 0, len(request.Keys))
	seen := make(map[string]struct{}, len(request.Keys))
	missing := 0
	for _, key := range request.Keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 {
			continue
		}
		switch parts[0] {
		case "item":
			if store.itemsByID[id] != nil {
				references = append(references, favoriteReference{kind: "item", id: id})
			} else {
				missing++
			}
		case "monster":
			if store.monstersByID[id] != nil && monsterVisible(rt, id) {
				references = append(references, favoriteReference{kind: "monster", id: id})
			} else {
				missing++
			}
		default:
			missing++
		}
	}

	page := 1
	pageSize := len(references)
	if !legacyRequest {
		page = max(1, request.Page)
		pageSize = request.PageSize
		if pageSize < 1 {
			pageSize = 24
		}
		pageSize = min(pageSize, 50)
	}
	pages := 1
	if pageSize > 0 {
		pages = max(1, int(math.Ceil(float64(len(references))/float64(pageSize))))
	}
	if page > pages {
		page = pages
	}
	start := 0
	end := len(references)
	if !legacyRequest && pageSize > 0 {
		start = min(len(references), (page-1)*pageSize)
		end = min(len(references), start+pageSize)
	}
	pageReferences := references[start:end]
	pageRows := make([]map[string]any, 0, len(pageReferences))
	for _, reference := range pageReferences {
		switch reference.kind {
		case "item":
			row := itemSummary(store.itemsByID[reference.id])
			row["kind"] = "item"
			pageRows = append(pageRows, row)
		case "monster":
			row := monsterSummary(store.monstersByID[reference.id])
			row["kind"] = "monster"
			pageRows = append(pageRows, row)
		}
	}
	items := make([]map[string]any, 0, len(pageRows))
	monsters := make([]map[string]any, 0, len(pageRows))
	for _, row := range pageRows {
		switch row["kind"] {
		case "item":
			items = append(items, row)
		case "monster":
			monsters = append(monsters, row)
		}
	}
	writeJSON(w, map[string]any{
		"rows":      pageRows,
		"items":     items,
		"monsters":  monsters,
		"total":     len(references),
		"page":      page,
		"pageSize":  pageSize,
		"pages":     pages,
		"missing":   missing,
		"totalKeys": len(seen),
	})
}
