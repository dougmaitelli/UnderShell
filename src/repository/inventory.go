package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"sshrpg/src/domain"
	"sshrpg/src/persistence/entity"
)

type InventoryRepository interface {
	FindOrCreate(context.Context, int64) (*domain.Inventory, error)
}

type BunInventoryRepository struct {
	db bun.IDB
}

func NewInventoryRepository(db bun.IDB) *BunInventoryRepository {
	return &BunInventoryRepository{db: db}
}

func (r *BunInventoryRepository) FindOrCreate(
	ctx context.Context,
	characterID int64,
) (*domain.Inventory, error) {
	record := &entity.Inventory{CharacterID: characterID}
	err := r.db.NewSelect().Model(record).WherePK().Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		record.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		if _, err := r.db.NewInsert().Model(record).Ignore().Exec(ctx); err != nil {
			return nil, fmt.Errorf("create inventory: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("find inventory: %w", err)
	}

	items := make([]entity.InventoryItem, 0)
	if err := r.db.NewSelect().
		Model(&items).
		Where("character_id = ?", characterID).
		Order("slot ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("find inventory items: %w", err)
	}

	inventory := &domain.Inventory{
		CharacterID: characterID,
		Items:       make([]domain.InventoryItem, len(items)),
	}
	for index, item := range items {
		inventory.Items[index] = domain.InventoryItem{
			Slot: item.Slot, ItemKey: item.ItemKey, Quantity: item.Quantity,
		}
	}
	return inventory, nil
}
