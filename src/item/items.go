// Package item loads and exposes the game's item definitions.
package item

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/content"
)

type Type string

const (
	TypeConsumable Type = "consumable"
	TypeEquipment  Type = "equipment"
	TypeMaterial   Type = "material"
)

type EquipmentSlot string

const (
	SlotHelmet EquipmentSlot = "helmet"
	SlotWeapon EquipmentSlot = "weapon"
	SlotArmor  EquipmentSlot = "armor"
	SlotBoots  EquipmentSlot = "boots"
	SlotGloves EquipmentSlot = "gloves"
	SlotLegs   EquipmentSlot = "legs"
)

type EffectType string

const (
	EffectRestoreHealth EffectType = "restore_health"
)

type Effect struct {
	Type   EffectType `json:"type"`
	Amount int        `json:"amount"`
}

type EquipmentStats struct {
	Attack   int `json:"attack,omitempty"`
	Defense  int `json:"defense,omitempty"`
	Vitality int `json:"vitality,omitempty"`
}

func (s EquipmentStats) IsZero() bool {
	return s.Attack == 0 && s.Defense == 0 && s.Vitality == 0
}

type Definition struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Type          Type           `json:"type"`
	EquipmentSlot EquipmentSlot  `json:"equipment_slot"`
	Stats         EquipmentStats `json:"stats,omitempty"`
	Effects       []Effect       `json:"effects,omitempty"`
	MaxStack      int            `json:"max_stack"`
}

type Items struct {
	items map[string]*Definition
	order []string
}

func LoadItems(directory string) (*Items, error) {
	definitions, err := content.LoadDefinitions[Definition](directory, "item")
	if err != nil {
		return nil, err
	}
	return NewItems(definitions)
}

func NewItems(definitions []Definition) (*Items, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one item is required")
	}

	items := &Items{
		items: make(map[string]*Definition, len(definitions)),
		order: make([]string, 0, len(definitions)),
	}
	for index, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		definition.Description = strings.TrimSpace(definition.Description)
		if err := validateDefinition(definition); err != nil {
			return nil, fmt.Errorf("item %d (%q): %w", index, definition.ID, err)
		}
		if _, exists := items.items[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate item ID %q", definition.ID)
		}
		items.items[definition.ID] = &definition
		items.order = append(items.order, definition.ID)
	}
	return items, nil
}

func (i *Items) Item(id string) (*Definition, bool) {
	definition, ok := i.items[id]
	return definition, ok
}

func (i *Items) All() []Definition {
	definitions := make([]Definition, 0, len(i.order))
	for _, id := range i.order {
		definitions = append(definitions, *i.items[id])
	}
	return definitions
}

func (i *Items) Len() int {
	return len(i.items)
}

func validateDefinition(definition Definition) error {
	if definition.ID == "" || definition.Name == "" {
		return errors.New("id and name are required")
	}
	for _, character := range definition.ID {
		if !((character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-') {
			return errors.New("id must contain only lowercase letters, numbers, underscores, or hyphens")
		}
	}
	for _, value := range []string{definition.Name, definition.Description} {
		for _, character := range value {
			if !unicode.IsPrint(character) || unicode.IsControl(character) {
				return errors.New("name and description must contain only printable characters")
			}
		}
	}
	if definition.MaxStack < 1 {
		return errors.New("max_stack must be at least 1")
	}
	if definition.Stats.Attack < 0 ||
		definition.Stats.Defense < 0 ||
		definition.Stats.Vitality < 0 {
		return errors.New("equipment stats cannot be negative")
	}
	switch definition.Type {
	case TypeConsumable:
		if definition.EquipmentSlot != "" {
			return fmt.Errorf(
				"%s items cannot define equipment_slot",
				definition.Type,
			)
		}
		if len(definition.Effects) == 0 {
			return errors.New("consumable items require at least one effect")
		}
		if !definition.Stats.IsZero() {
			return errors.New("consumable items cannot define stats")
		}
	case TypeMaterial:
		if definition.EquipmentSlot != "" {
			return fmt.Errorf(
				"%s items cannot define equipment_slot",
				definition.Type,
			)
		}
		if len(definition.Effects) > 0 {
			return errors.New("material items cannot define effects")
		}
		if !definition.Stats.IsZero() {
			return errors.New("material items cannot define stats")
		}
	case TypeEquipment:
		switch definition.EquipmentSlot {
		case SlotHelmet, SlotWeapon, SlotArmor,
			SlotBoots, SlotGloves, SlotLegs:
		default:
			return errors.New(
				"equipment items require a valid equipment_slot",
			)
		}
		if definition.MaxStack != 1 {
			return errors.New("equipment items must have max_stack 1")
		}
		if len(definition.Effects) > 0 {
			return errors.New("equipment items cannot define effects")
		}
	default:
		return fmt.Errorf("unsupported item type %q", definition.Type)
	}
	seenEffects := make(map[EffectType]bool, len(definition.Effects))
	for _, effect := range definition.Effects {
		if seenEffects[effect.Type] {
			return fmt.Errorf("duplicate effect type %q", effect.Type)
		}
		seenEffects[effect.Type] = true
		switch effect.Type {
		case EffectRestoreHealth:
			if effect.Amount < 1 {
				return errors.New(
					"restore_health effect amount must be at least 1",
				)
			}
		default:
			return fmt.Errorf("unsupported effect type %q", effect.Type)
		}
	}
	return nil
}
