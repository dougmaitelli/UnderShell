package content

import (
	"os"
	"path/filepath"
	"testing"
)

type testDefinition struct {
	ID string `json:"id"`
}

func TestLoadDefinitionsReadsJSONFilesInFilenameOrder(t *testing.T) {
	directory := t.TempDir()
	for name, contents := range map[string]string{
		"02-second.json": `{"id":"second"}`,
		"01-first.json":  `{"id":"first"}`,
		"README.md":      "ignored",
	} {
		if err := os.WriteFile(
			filepath.Join(directory, name), []byte(contents), 0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	definitions, err := LoadDefinitions[testDefinition](directory, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 2 ||
		definitions[0].ID != "first" ||
		definitions[1].ID != "second" {
		t.Fatalf("definitions = %#v", definitions)
	}
}

func TestLoadDefinitionsRejectsInvalidDefinitionFile(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(directory, "invalid.json"),
		[]byte(`{"id":"invalid","unknown":true}`),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadDefinitions[testDefinition](directory, "test"); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}
