// Package domain contains game-facing models and validation.
package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidCharacterName = errors.New("name must be 3-20 printable characters")

// Character is the game-facing representation of a character. Database
// identity and ORM metadata intentionally do not belong in this model.
type Character struct {
	ID   int64
	Name string
	X    int
	Y    int
}

func ValidateCharacterName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) < 3 || utf8.RuneCountInString(value) > 20 {
		return "", ErrInvalidCharacterName
	}
	for _, r := range value {
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return "", ErrInvalidCharacterName
		}
	}
	return value, nil
}
