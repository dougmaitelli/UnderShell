package admin

import (
	"errors"
	"strings"
	"unicode"
)

func splitArguments(value string) ([]string, error) {
	var (
		fields  []string
		current strings.Builder
		quote   rune
		escaped bool
		started bool
	)
	flush := func() {
		if !started {
			return
		}
		fields = append(fields, current.String())
		current.Reset()
		started = false
	}
	for _, character := range value {
		switch {
		case escaped:
			current.WriteRune(character)
			escaped = false
			started = true
		case character == '\\':
			escaped = true
			started = true
		case quote != 0 && character == quote:
			quote = 0
		case quote != 0:
			current.WriteRune(character)
			started = true
		case character == '\'' || character == '"':
			quote = character
			started = true
		case unicode.IsSpace(character):
			flush()
		default:
			current.WriteRune(character)
			started = true
		}
	}
	if escaped {
		return nil, errors.New("unfinished escape sequence")
	}
	if quote != 0 {
		return nil, errors.New("unterminated quoted argument")
	}
	flush()
	return fields, nil
}
