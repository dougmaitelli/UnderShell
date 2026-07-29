// Package content provides shared loading helpers for game content definitions.
package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoadDefinitions decodes one definition from every JSON file in directory.
// os.ReadDir returns entries ordered by filename, making registry order stable.
func LoadDefinitions[T any](directory, kind string) ([]T, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read %s definitions directory %s: %w", kind, directory, err)
	}

	definitions := make([]T, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		definition, err := decodeDefinition[T](path)
		if err != nil {
			return nil, fmt.Errorf("decode %s definition %s: %w", kind, path, err)
		}
		definitions = append(definitions, definition)
	}
	if len(definitions) == 0 {
		return nil, fmt.Errorf("no JSON %s definitions found in %s", kind, directory)
	}
	return definitions, nil
}

func decodeDefinition[T any](path string) (T, error) {
	var definition T
	file, err := os.Open(path)
	if err != nil {
		return definition, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return definition, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return definition, err
	}
	return definition, nil
}
