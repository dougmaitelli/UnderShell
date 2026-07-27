// Package entity contains ORM persistence models.
package entity

import "github.com/uptrace/bun"

// Character is the persistence representation of a character. It contains
// storage-only SSH identity and ORM metadata that are not exposed to the game.
type Character struct {
	bun.BaseModel `bun:"table:characters,alias:character"`

	ID             int64  `bun:"id,pk,autoincrement"`
	KeyFingerprint string `bun:"key_fingerprint,notnull,unique"`
	PublicKeyType  string `bun:"public_key_type,notnull"`
	PublicKey      string `bun:"public_key,notnull"`
	Name           string `bun:"name,notnull,unique,type:TEXT COLLATE NOCASE"`
	CreatedAt      string `bun:"created_at,notnull"`
	LastSeenAt     string `bun:"last_seen_at,notnull"`
}
