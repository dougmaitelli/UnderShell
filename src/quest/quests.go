// Package quest loads and exposes the game's quest definitions.
package quest

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sshrpg/src/content"
	"sshrpg/src/enemy"
	"sshrpg/src/item"
)

type Objective struct {
	Item     *item.Definition
	Quantity int
}

type Reward struct {
	Gold int
}

type Dialogue struct {
	Offer      string
	InProgress string
	Ready      string
	Completed  string
}

type Definition struct {
	ID          string
	Name        string
	Description string
	Objective   Objective
	Reward      Reward
	Dialogue    Dialogue
}

type objectiveFile struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

type rewardFile struct {
	Gold int `json:"gold"`
}

type dialogueFile struct {
	Offer      string `json:"offer"`
	InProgress string `json:"in_progress"`
	Ready      string `json:"ready"`
	Completed  string `json:"completed"`
}

type definitionFile struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Objective   objectiveFile `json:"objective"`
	Reward      rewardFile    `json:"reward"`
	Dialogue    dialogueFile  `json:"dialogue"`
}

type Quests struct {
	quests map[string]*Definition
	order  []string
}

func LoadQuests(directory string, items *item.Items) (*Quests, error) {
	if items == nil {
		return nil, errors.New("item definitions are required")
	}
	sources, err := content.LoadDefinitions[definitionFile](directory, "quest")
	if err != nil {
		return nil, err
	}
	definitions := make([]Definition, len(sources))
	for index, source := range sources {
		itemDefinition, ok := items.Item(strings.TrimSpace(source.Objective.ItemID))
		if !ok {
			return nil, fmt.Errorf(
				"quest %q references unknown item %q",
				source.ID, source.Objective.ItemID,
			)
		}
		definitions[index] = Definition{
			ID: source.ID, Name: source.Name, Description: source.Description,
			Objective: Objective{
				Item: itemDefinition, Quantity: source.Objective.Quantity,
			},
			Reward: Reward{Gold: source.Reward.Gold},
			Dialogue: Dialogue{
				Offer:      source.Dialogue.Offer,
				InProgress: source.Dialogue.InProgress,
				Ready:      source.Dialogue.Ready,
				Completed:  source.Dialogue.Completed,
			},
		}
	}
	return NewQuests(definitions)
}

func NewQuests(definitions []Definition) (*Quests, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one quest is required")
	}
	quests := &Quests{
		quests: make(map[string]*Definition, len(definitions)),
		order:  make([]string, 0, len(definitions)),
	}
	for index, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		definition.Description = strings.TrimSpace(definition.Description)
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
		quests.quests[definition.ID] = &definition
		quests.order = append(quests.order, definition.ID)
	}
	return quests, nil
}

func (q *Quests) ValidateObjectives(enemies *enemy.Enemies) error {
	if enemies == nil {
		return errors.New("enemies are required")
	}
	droppedItems := make(map[string]bool)
	for _, definition := range enemies.All() {
		for _, drop := range definition.Drops {
			droppedItems[drop.Item.ID] = true
		}
	}
	for _, id := range q.order {
		definition := q.quests[id]
		if !droppedItems[definition.Objective.Item.ID] {
			return fmt.Errorf(
				"quest %q objective item %q is not dropped by any enemy",
				definition.ID, definition.Objective.Item.ID,
			)
		}
	}
	return nil
}

func (q *Quests) Quest(id string) (*Definition, bool) {
	definition, ok := q.quests[id]
	return definition, ok
}

func (q *Quests) All() []*Definition {
	definitions := make([]*Definition, 0, len(q.order))
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
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '_' && character != '-' {
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
	if definition.Objective.Item == nil {
		return errors.New("objective requires an item reference")
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
