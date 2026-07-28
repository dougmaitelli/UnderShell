// Package npc defines configured non-player characters and their behavior-specific data.
package npc

import (
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/item"
	"sshrpg/src/quest"
)

type Type string

const (
	TypeShop       Type = "shop"
	TypeQuestGiver Type = "quest_giver"
)

type Config struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Type     Type             `json:"type"`
	X        int              `json:"x"`
	Y        int              `json:"y"`
	Stock    []ShopItemConfig `json:"stock"`
	QuestIDs []string         `json:"quests"`
}

type ShopItemConfig struct {
	ItemID    string `json:"item_id"`
	BuyPrice  int    `json:"buy_price"`
	SellPrice int    `json:"sell_price"`
}

type Definition struct {
	ID     string
	Name   string
	Type   Type
	X      int
	Y      int
	Stock  []ShopItem
	Quests []*quest.Definition
}

type ShopItem struct {
	Item      *item.Definition
	BuyPrice  int
	SellPrice int
}

func Clone(definitions []Definition) []Definition {
	clones := append([]Definition(nil), definitions...)
	for index := range clones {
		clones[index].Stock = append([]ShopItem(nil), definitions[index].Stock...)
		clones[index].Quests = append(
			[]*quest.Definition(nil), definitions[index].Quests...,
		)
	}
	return clones
}

func Resolve(
	configs []Config,
	items *item.Items,
	quests *quest.Quests,
) ([]Definition, error) {
	definitions := make([]Definition, len(configs))
	ids := make(map[string]bool, len(configs))
	for index := range configs {
		config := &configs[index]
		definition := &definitions[index]
		definition.ID = config.ID
		definition.Name = config.Name
		definition.Type = config.Type
		definition.X, definition.Y = config.X, config.Y
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		if definition.ID == "" || definition.Name == "" {
			return nil, fmt.Errorf("NPC %d requires id and name", index)
		}
		for _, character := range definition.ID {
			if !((character >= 'a' && character <= 'z') ||
				(character >= '0' && character <= '9') ||
				character == '_' || character == '-') {
				return nil, fmt.Errorf("NPC %q id has unsupported characters", definition.ID)
			}
		}
		for _, character := range definition.Name {
			if unicode.IsControl(character) || !unicode.IsPrint(character) {
				return nil, fmt.Errorf("NPC %q name must be printable", definition.ID)
			}
		}
		if ids[definition.ID] {
			return nil, fmt.Errorf("duplicate NPC ID %q", definition.ID)
		}
		ids[definition.ID] = true
		switch definition.Type {
		case TypeShop:
			if len(config.QuestIDs) > 0 {
				return nil, fmt.Errorf("shop NPC %q cannot define quests", definition.ID)
			}
			stock, err := resolveShop(definition.ID, config.Stock, items)
			if err != nil {
				return nil, err
			}
			definition.Stock = stock
		case TypeQuestGiver:
			if len(config.Stock) > 0 {
				return nil, fmt.Errorf("quest giver NPC %q cannot define stock", definition.ID)
			}
			resolvedQuests, err := resolveQuests(
				definition.ID, config.QuestIDs, quests,
			)
			if err != nil {
				return nil, err
			}
			definition.Quests = resolvedQuests
		default:
			return nil, fmt.Errorf(
				"NPC %q has unsupported type %q",
				definition.ID, definition.Type,
			)
		}
	}
	return definitions, nil
}

func resolveShop(
	npcID string,
	configs []ShopItemConfig,
	items *item.Items,
) ([]ShopItem, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("shop NPC %q requires stock", npcID)
	}
	if items == nil {
		return nil, fmt.Errorf("shop NPC %q requires item definitions", npcID)
	}
	stock := make([]ShopItem, len(configs))
	stockIDs := make(map[string]bool, len(configs))
	for index, config := range configs {
		itemID := strings.TrimSpace(config.ItemID)
		if itemID == "" {
			return nil, fmt.Errorf("NPC %q stock %d requires item_id", npcID, index)
		}
		if stockIDs[itemID] {
			return nil, fmt.Errorf("NPC %q has duplicate stock item %q", npcID, itemID)
		}
		stockIDs[itemID] = true
		if config.BuyPrice < 1 || config.SellPrice < 1 {
			return nil, fmt.Errorf("NPC %q stock %q prices must be positive", npcID, itemID)
		}
		if config.SellPrice > config.BuyPrice {
			return nil, fmt.Errorf(
				"NPC %q stock %q sell_price cannot exceed buy_price",
				npcID, itemID,
			)
		}
		itemDefinition, ok := items.Item(itemID)
		if !ok {
			return nil, fmt.Errorf(
				"NPC %q stock %d references unknown item %q",
				npcID, index, itemID,
			)
		}
		stock[index] = ShopItem{
			Item:     itemDefinition,
			BuyPrice: config.BuyPrice, SellPrice: config.SellPrice,
		}
	}
	return stock, nil
}

func resolveQuests(
	npcID string,
	questIDs []string,
	quests *quest.Quests,
) ([]*quest.Definition, error) {
	if len(questIDs) == 0 {
		return nil, fmt.Errorf("quest giver NPC %q requires quests", npcID)
	}
	if quests == nil {
		return nil, fmt.Errorf("quest giver NPC %q requires quest definitions", npcID)
	}
	resolved := make([]*quest.Definition, len(questIDs))
	ids := make(map[string]bool, len(questIDs))
	for index, rawID := range questIDs {
		questID := strings.TrimSpace(rawID)
		if questID == "" {
			return nil, fmt.Errorf("NPC %q quest %d requires an ID", npcID, index)
		}
		if ids[questID] {
			return nil, fmt.Errorf("NPC %q has duplicate quest %q", npcID, questID)
		}
		ids[questID] = true
		definition, ok := quests.Quest(questID)
		if !ok {
			return nil, fmt.Errorf(
				"NPC %q quest %d references unknown quest %q",
				npcID, index, questID,
			)
		}
		resolved[index] = definition
	}
	return resolved, nil
}
