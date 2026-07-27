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
	UpdateLocation(context.Context, int64, string, int, int) error
	UpdateProgress(context.Context, int64, int, int64, int) error
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

	location := &entity.CharacterLocation{CharacterID: record.ID}
	err = r.db.NewSelect().
		Model(location).
		WherePK().
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		location = nil
	} else if err != nil {
		return nil, fmt.Errorf("find character location: %w", err)
	}
	progress := &entity.CharacterProgress{CharacterID: record.ID}
	err = r.db.NewSelect().Model(progress).WherePK().Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		progress = nil
	} else if err != nil {
		return nil, fmt.Errorf("find character progress: %w", err)
	}
	return toDomain(record, location, progress), nil
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
	return toDomain(record, nil, nil), nil
}

func (r *BunCharacterRepository) UpdateLocation(
	ctx context.Context,
	id int64,
	areaID string,
	x, y int,
) error {
	location := &entity.CharacterLocation{CharacterID: id}
	err := r.db.NewSelect().
		Model(location).
		WherePK().
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		location = &entity.CharacterLocation{
			CharacterID: id,
			AreaID:      areaID,
			X:           x,
			Y:           y,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		if _, err := r.db.NewInsert().Model(location).Exec(ctx); err != nil {
			return fmt.Errorf("create character location: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("find character location for update: %w", err)
	}

	location.AreaID = areaID
	location.X = x
	location.Y = y
	location.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.NewUpdate().
		Model(location).
		Column("area_id", "x", "y", "updated_at").
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

func (r *BunCharacterRepository) UpdateProgress(
	ctx context.Context,
	id int64,
	level int,
	experience int64,
	skillPoints int,
) error {
	if level < 1 || experience < 0 || skillPoints < 0 {
		return errors.New("invalid character progress")
	}
	progress := &entity.CharacterProgress{
		CharacterID: id,
		Level:       level, Experience: experience, SkillPoints: skillPoints,
	}
	if _, err := r.db.NewInsert().
		Model(progress).
		On("CONFLICT (character_id) DO UPDATE").
		Set("level = EXCLUDED.level").
		Set("experience = EXCLUDED.experience").
		Set("skill_points = EXCLUDED.skill_points").
		Exec(ctx); err != nil {
		return fmt.Errorf("update character progress: %w", err)
	}
	return nil
}

func toDomain(
	record *entity.Character,
	location *entity.CharacterLocation,
	progress *entity.CharacterProgress,
) *domain.Character {
	character := &domain.Character{ID: record.ID, Name: record.Name, Level: 1}
	if location != nil {
		character.AreaID = location.AreaID
		character.X = location.X
		character.Y = location.Y
	}
	if progress != nil {
		character.Level = progress.Level
		character.Experience = progress.Experience
		character.SkillPoints = progress.SkillPoints
	}
	return character
}
