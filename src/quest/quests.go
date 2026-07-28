// Package quest loads and exposes the game's quest definitions.
package quest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

type Objective struct {
	ItemID   string           `json:"item_id"`
	Quantity int              `json:"quantity"`
	Item     *item.Definition `json:"-"`
}

type Reward struct {
	Gold int `json:"gold"`
}

type Dialogue struct {
	Offer      string `json:"offer"`
	InProgress string `json:"in_progress"`
	Ready      string `json:"ready"`
	Completed  string `json:"completed"`
}

type Definition struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Objective   Objective `json:"objective"`
	Reward      Reward    `json:"reward"`
	Dialogue    Dialogue  `json:"dialogue"`
}

type questsFile struct {
	Quests []Definition `json:"quests"`
}

type Quests struct {
	quests map[string]Definition
	order  []string
}

func LoadQuests(path string) (*Quests, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open quest definitions %s: %w", path, err)
	}
	defer file.Close()

	var contents questsFile
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contents); err != nil {
		return nil, fmt.Errorf("decode quest definitions %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, fmt.Errorf("decode quest definitions %s: %w", path, err)
	}
	return NewQuests(contents.Quests)
}

func NewQuests(definitions []Definition) (*Quests, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one quest is required")
	}
	quests := &Quests{
		quests: make(map[string]Definition, len(definitions)),
		order:  make([]string, 0, len(definitions)),
	}
	for index, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		definition.Description = strings.TrimSpace(definition.Description)
		definition.Objective.ItemID = strings.TrimSpace(definition.Objective.ItemID)
		definition.Dialogue.Offer = strings.TrimSpace(definition.Dialogue.Offer)
		definition.Dialogue.InProgress = strings.TrimSpace(definition.Dialogue.InProgress)
		definition.Dialogue.Ready = strings.TrimSpace(definition.Dialogue.Ready)
		definition.Dialogue.Completed = strings.TrimSpace(definition.Dialogue.Completed)
		if err := validateDefinition(definition); err != nil {
			return nil, fmt.Errorf("quest %d (%q): %w", index, definition.ID, err)
		}
		if _, exists := quests.quests[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate quest ID %q", definition.ID)
		}
		quests.quests[definition.ID] = definition
		quests.order = append(quests.order, definition.ID)
	}
	return quests, nil
}

func (q *Quests) ResolveItems(items *item.Items) error {
	if items == nil {
		return errors.New("items are required")
	}
	for _, id := range q.order {
		definition := q.quests[id]
		itemDefinition, ok := items.Item(definition.Objective.ItemID)
		if !ok {
			return fmt.Errorf(
				"quest %q references unknown item %q",
				definition.ID, definition.Objective.ItemID,
			)
		}
		definition.Objective.Item = itemDefinition
		q.quests[id] = definition
	}
	return nil
}

func (q *Quests) ValidateObjectives(enemies *enemy.Enemies) error {
	if enemies == nil {
		return errors.New("enemies are required")
	}
	droppedItems := make(map[string]bool)
	for _, definition := range enemies.All() {
		for _, drop := range definition.Drops {
			droppedItems[drop.ItemID] = true
		}
	}
	for _, id := range q.order {
		definition := q.quests[id]
		if !droppedItems[definition.Objective.ItemID] {
			return fmt.Errorf(
				"quest %q objective item %q is not dropped by any enemy",
				definition.ID, definition.Objective.ItemID,
			)
		}
	}
	return nil
}

func (q *Quests) Quest(id string) (Definition, bool) {
	definition, ok := q.quests[id]
	return definition, ok
}

func (q *Quests) All() []Definition {
	definitions := make([]Definition, 0, len(q.order))
	for _, id := range q.order {
		definitions = append(definitions, q.quests[id])
	}
	return definitions
}

func (q *Quests) Len() int { return len(q.quests) }

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
	if definition.Objective.ItemID == "" {
		return errors.New("objective.item_id is required")
	}
	if definition.Objective.Quantity < 1 {
		return errors.New("objective.quantity must be at least 1")
	}
	if definition.Reward.Gold < 0 {
		return errors.New("reward.gold cannot be negative")
	}
	dialogues := []struct {
		name  string
		value string
	}{
		{"offer", definition.Dialogue.Offer},
		{"in_progress", definition.Dialogue.InProgress},
		{"ready", definition.Dialogue.Ready},
		{"completed", definition.Dialogue.Completed},
	}
	for _, dialogue := range dialogues {
		if dialogue.value == "" {
			return fmt.Errorf("dialogue.%s is required", dialogue.name)
		}
		for _, character := range dialogue.value {
			if !unicode.IsPrint(character) || unicode.IsControl(character) {
				return fmt.Errorf(
					"dialogue.%s must contain only printable characters",
					dialogue.name,
				)
			}
		}
	}
	return nil
}
