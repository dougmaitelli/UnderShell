// Package enemy loads and exposes the game's enemy definitions.
package enemy

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/content"
	"sshrpg/src/item"
)

type Drop struct {
	Item   *item.Definition
	Chance float64
}

type Definition struct {
	ID          string
	Name        string
	Description string
	Visual      []string
	Health      int
	Damage      int
	Experience  int64
	Drops       []Drop
}

type dropFile struct {
	ItemID string  `json:"item_id"`
	Chance float64 `json:"chance"`
}

type definitionFile struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Visual      []string   `json:"visual"`
	Health      int        `json:"health"`
	Damage      int        `json:"damage"`
	Experience  int64      `json:"experience"`
	Drops       []dropFile `json:"drops"`
}

type Enemies struct {
	enemies map[string]*Definition
	order   []string
}

func LoadEnemies(directory string, items *item.Items) (*Enemies, error) {
	if items == nil {
		return nil, errors.New("item definitions are required")
	}
	sources, err := content.LoadDefinitions[definitionFile](directory, "enemy")
	if err != nil {
		return nil, err
	}
	definitions := make([]Definition, len(sources))
	for index, source := range sources {
		definition := Definition{
			ID: source.ID, Name: source.Name, Description: source.Description,
			Visual: source.Visual, Health: source.Health, Damage: source.Damage,
			Experience: source.Experience,
			Drops:      make([]Drop, len(source.Drops)),
		}
		for dropIndex, sourceDrop := range source.Drops {
			itemDefinition, ok := items.Item(strings.TrimSpace(sourceDrop.ItemID))
			if !ok {
				return nil, fmt.Errorf(
					"enemy %q drop %d references unknown item %q",
					source.ID, dropIndex, sourceDrop.ItemID,
				)
			}
			definition.Drops[dropIndex] = Drop{
				Item: itemDefinition, Chance: sourceDrop.Chance,
			}
		}
		definitions[index] = definition
	}
	return NewEnemies(definitions)
}

func NewEnemies(definitions []Definition) (*Enemies, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one enemy is required")
	}
	enemies := &Enemies{
		enemies: make(map[string]*Definition, len(definitions)),
		order:   make([]string, 0, len(definitions)),
	}
	for index, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		definition.Description = strings.TrimSpace(definition.Description)
		definition.Visual = append([]string(nil), definition.Visual...)
		definition.Drops = append([]Drop(nil), definition.Drops...)
		if err := validateDefinition(definition); err != nil {
			return nil, fmt.Errorf("enemy %d (%q): %w", index, definition.ID, err)
		}
		if _, exists := enemies.enemies[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate enemy ID %q", definition.ID)
		}
		enemies.enemies[definition.ID] = &definition
		enemies.order = append(enemies.order, definition.ID)
	}
	return enemies, nil
}

func (e *Enemies) Enemy(id string) (*Definition, bool) {
	definition, ok := e.enemies[id]
	return definition, ok
}

func (e *Enemies) All() []*Definition {
	definitions := make([]*Definition, 0, len(e.order))
	for _, id := range e.order {
		definitions = append(definitions, e.enemies[id])
	}
	return definitions
}

func (e *Enemies) Len() int { return len(e.enemies) }

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
	if len(definition.Visual) == 0 || len(definition.Visual) > 5 {
		return errors.New("visual must contain between 1 and 5 rows")
	}
	hasVisibleCell := false
	for rowIndex, row := range definition.Visual {
		cells := []rune(row)
		if len(cells) == 0 || len(cells) > 15 {
			return fmt.Errorf("visual row %d must contain between 1 and 15 characters", rowIndex)
		}
		for _, cell := range cells {
			if cell < 0x20 || cell > 0x7e {
				return fmt.Errorf("visual row %d must contain only ASCII characters", rowIndex)
			}
			if cell != ' ' {
				hasVisibleCell = true
			}
		}
	}
	if !hasVisibleCell {
		return errors.New("visual must contain at least one non-space character")
	}
	if definition.Health < 1 {
		return errors.New("health must be at least 1")
	}
	if definition.Damage < 0 {
		return errors.New("damage cannot be negative")
	}
	if definition.Experience < 1 {
		return errors.New("experience must be at least 1")
	}
	for index, drop := range definition.Drops {
		if drop.Item == nil {
			return fmt.Errorf("drop %d requires an item reference", index)
		}
		if drop.Chance <= 0 || drop.Chance > 1 {
			return fmt.Errorf("drop %d chance must be greater than 0 and at most 1", index)
		}
	}
	return nil
}
