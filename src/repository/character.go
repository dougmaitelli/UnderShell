package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"sshrpg/src/domain"
	"sshrpg/src/persistence/entity"
)

var (
	ErrCharacterNameTaken = errors.New("that name is already taken")
	ErrCharacterKeyExists = errors.New("a character already exists for that SSH key")
)

type CreateCharacterParams struct {
	KeyFingerprint string
	PublicKeyType  string
	PublicKey      string
	Name           string
}

// CharacterRepository is the data-access contract used by the application.
// Its callers work only with domain models, never persistence entities.
type CharacterRepository interface {
	FindByFingerprint(context.Context, string) (*domain.Character, error)
	Create(context.Context, CreateCharacterParams) (*domain.Character, error)
	UpdatePosition(context.Context, int64, int, int) error
}

type BunCharacterRepository struct {
	db bun.IDB
}

func NewCharacterRepository(db bun.IDB) *BunCharacterRepository {
	return &BunCharacterRepository{db: db}
}

func (r *BunCharacterRepository) FindByFingerprint(
	ctx context.Context,
	fingerprint string,
) (*domain.Character, error) {
	record := new(entity.Character)
	err := r.db.NewSelect().
		Model(record).
		Where("key_fingerprint = ?", fingerprint).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find character by fingerprint: %w", err)
	}
	return toDomain(record), nil
}

func (r *BunCharacterRepository) Create(
	ctx context.Context,
	params CreateCharacterParams,
) (*domain.Character, error) {
	name, err := domain.ValidateCharacterName(params.Name)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	record := &entity.Character{
		KeyFingerprint: params.KeyFingerprint,
		PublicKeyType:  params.PublicKeyType,
		PublicKey:      params.PublicKey,
		Name:           name,
		CreatedAt:      now,
		LastSeenAt:     now,
	}
	_, err = r.db.NewInsert().Model(record).Exec(ctx)
	if err != nil {
		lower := strings.ToLower(err.Error())
		switch {
		case strings.Contains(lower, "characters.name"):
			return nil, ErrCharacterNameTaken
		case strings.Contains(lower, "characters.key_fingerprint"):
			return nil, ErrCharacterKeyExists
		default:
			return nil, fmt.Errorf("create character: %w", err)
		}
	}
	return toDomain(record), nil
}

func (r *BunCharacterRepository) UpdatePosition(
	ctx context.Context,
	id int64,
	x, y int,
) error {
	record := &entity.Character{
		ID: id, X: x, Y: y,
		LastSeenAt: time.Now().UTC().Format(time.RFC3339),
	}
	result, err := r.db.NewUpdate().
		Model(record).
		Column("x", "y", "last_seen_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update character position: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated row count: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("update character position: character %d not found", id)
	}
	return nil
}

func toDomain(record *entity.Character) *domain.Character {
	return &domain.Character{
		ID: record.ID, Name: record.Name, X: record.X, Y: record.Y,
	}
}
