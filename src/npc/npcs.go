// Package npc defines configured non-player characters and their behavior-specific data.
package npc

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/item"
)

type Type string

const TypeShop Type = "shop"

type Definition struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Type  Type       `json:"type"`
	X     int        `json:"x"`
	Y     int        `json:"y"`
	Stock []ShopItem `json:"stock"`
}

type ShopItem struct {
	ItemID    string `json:"item_id"`
	BuyPrice  int    `json:"buy_price"`
	SellPrice int    `json:"sell_price"`
	Name      string `json:"-"`
	MaxStack  int    `json:"-"`
}

func Clone(definitions []Definition) []Definition {
	clones := append([]Definition(nil), definitions...)
	for index := range clones {
		clones[index].Stock = append([]ShopItem(nil), definitions[index].Stock...)
	}
	return clones
}

func Validate(definitions []Definition) error {
	ids := make(map[string]bool, len(definitions))
	for index := range definitions {
		definition := &definitions[index]
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		if definition.ID == "" || definition.Name == "" {
			return fmt.Errorf("NPC %d requires id and name", index)
		}
		for _, character := range definition.ID {
			if !((character >= 'a' && character <= 'z') ||
				(character >= '0' && character <= '9') ||
				character == '_' || character == '-') {
				return fmt.Errorf("NPC %q id has unsupported characters", definition.ID)
			}
		}
		for _, character := range definition.Name {
			if unicode.IsControl(character) || !unicode.IsPrint(character) {
				return fmt.Errorf("NPC %q name must be printable", definition.ID)
			}
		}
		if ids[definition.ID] {
			return fmt.Errorf("duplicate NPC ID %q", definition.ID)
		}
		ids[definition.ID] = true
		if definition.Type != TypeShop {
			return fmt.Errorf("NPC %q has unsupported type %q", definition.ID, definition.Type)
		}
		if err := validateShop(definition); err != nil {
			return err
		}
	}
	return nil
}

func ResolveItems(definitions []Definition, items *item.Items) error {
	if items == nil {
		return errors.New("items are required")
	}
	for npcIndex := range definitions {
		definition := &definitions[npcIndex]
		for stockIndex := range definition.Stock {
			stock := &definition.Stock[stockIndex]
			itemDefinition, ok := items.Item(stock.ItemID)
			if !ok {
				return fmt.Errorf(
					"NPC %q stock %d references unknown item %q",
					definition.ID, stockIndex, stock.ItemID,
				)
			}
			stock.Name = itemDefinition.Name
			stock.MaxStack = itemDefinition.MaxStack
		}
	}
	return nil
}

func validateShop(definition *Definition) error {
	if len(definition.Stock) == 0 {
		return fmt.Errorf("shop NPC %q requires stock", definition.ID)
	}
	stockIDs := make(map[string]bool, len(definition.Stock))
	for index := range definition.Stock {
		stock := &definition.Stock[index]
		stock.ItemID = strings.TrimSpace(stock.ItemID)
		if stock.ItemID == "" {
			return fmt.Errorf("NPC %q stock %d requires item_id", definition.ID, index)
		}
		if stockIDs[stock.ItemID] {
			return fmt.Errorf("NPC %q has duplicate stock item %q", definition.ID, stock.ItemID)
		}
		stockIDs[stock.ItemID] = true
		if stock.BuyPrice < 1 || stock.SellPrice < 1 {
			return fmt.Errorf("NPC %q stock %q prices must be positive", definition.ID, stock.ItemID)
		}
		if stock.SellPrice > stock.BuyPrice {
			return fmt.Errorf(
				"NPC %q stock %q sell_price cannot exceed buy_price",
				definition.ID, stock.ItemID,
			)
		}
	}
	return nil
}
