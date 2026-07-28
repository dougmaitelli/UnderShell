// Package domain contains game-facing models and validation.
package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidCharacterName = errors.New("name must be 3-20 printable characters")
var ErrInvalidCharacterRole = errors.New("role must be user or admin")

const DefaultStartingGold = 100

type CharacterRole string

const (
	CharacterRoleUser  CharacterRole = "user"
	CharacterRoleAdmin CharacterRole = "admin"
)

func ValidateCharacterRole(role CharacterRole) error {
	switch role {
	case CharacterRoleUser, CharacterRoleAdmin:
		return nil
	default:
		return ErrInvalidCharacterRole
	}
}

// Character is the game-facing representation of a character. Database
// identity and ORM metadata intentionally do not belong in this model.
type Character struct {
	ID          int64
	Name        string
	Role        CharacterRole
	AreaID      string
	X           int
	Y           int
	Level       int
	Experience  int64
	SkillPoints int
	Attack      int
	Defense     int
	Vitality    int
	Gold        int
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
