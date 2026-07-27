// Package item loads and exposes the game's item definitions.
package item

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

type Definition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxStack    int    `json:"max_stack"`
}

type catalogFile struct {
	Items []Definition `json:"items"`
}

type Catalog struct {
	items map[string]Definition
	order []string
}

func LoadCatalog(path string) (*Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open item catalog %s: %w", path, err)
	}
	defer file.Close()

	var contents catalogFile
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contents); err != nil {
		return nil, fmt.Errorf("decode item catalog %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, fmt.Errorf("decode item catalog %s: %w", path, err)
	}
	return NewCatalog(contents.Items)
}

func NewCatalog(definitions []Definition) (*Catalog, error) {
	if len(definitions) == 0 {
		return nil, errors.New("at least one item is required")
	}

	catalog := &Catalog{
		items: make(map[string]Definition, len(definitions)),
		order: make([]string, 0, len(definitions)),
	}
	for index, definition := range definitions {
		definition.ID = strings.TrimSpace(definition.ID)
		definition.Name = strings.TrimSpace(definition.Name)
		definition.Description = strings.TrimSpace(definition.Description)
		if err := validateDefinition(definition); err != nil {
			return nil, fmt.Errorf("item %d (%q): %w", index, definition.ID, err)
		}
		if _, exists := catalog.items[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate item ID %q", definition.ID)
		}
		catalog.items[definition.ID] = definition
		catalog.order = append(catalog.order, definition.ID)
	}
	return catalog, nil
}

func (c *Catalog) Item(id string) (Definition, bool) {
	definition, ok := c.items[id]
	return definition, ok
}

func (c *Catalog) All() []Definition {
	definitions := make([]Definition, 0, len(c.order))
	for _, id := range c.order {
		definitions = append(definitions, c.items[id])
	}
	return definitions
}

func (c *Catalog) Len() int {
	return len(c.items)
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
	return nil
}
