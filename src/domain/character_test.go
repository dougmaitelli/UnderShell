package domain

import "testing"

func TestValidateCharacterName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"Aria", true},
		{"  Rowan  ", true},
		{"ab", false},
		{"name\x1b[2J", false},
		{"this name is much too long", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateCharacterName(test.name)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}
