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
	AddItem(context.Context, int64, string, int) (*domain.Inventory, error)
}

func (r *BunInventoryRepository) AddItem(
	ctx context.Context,
	characterID int64,
	itemKey string,
	maxStack int,
) (*domain.Inventory, error) {
	if maxStack < 1 {
		return nil, errors.New("max stack must be at least 1")
	}
	stack := &entity.InventoryItem{}
	err := r.db.NewSelect().
		Model(stack).
		Where("character_id = ?", characterID).
		Where("item_key = ?", itemKey).
		Where("quantity < ?", maxStack).
		Order("slot ASC").
		Limit(1).
		Scan(ctx)
	switch {
	case err == nil:
		if _, err := r.db.NewUpdate().
			Model(stack).
			Column("quantity").
			Set("quantity = quantity + 1").
			WherePK().
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("increase inventory item: %w", err)
		}
	case errors.Is(err, sql.ErrNoRows):
		var nextSlot int
		if err := r.db.NewSelect().
			Model((*entity.InventoryItem)(nil)).
			ColumnExpr("COALESCE(MAX(slot), 0) + 1").
			Where("character_id = ?", characterID).
			Scan(ctx, &nextSlot); err != nil {
			return nil, fmt.Errorf("find next inventory slot: %w", err)
		}
		stack = &entity.InventoryItem{
			CharacterID: characterID,
			Slot:        nextSlot, ItemKey: itemKey, Quantity: 1,
		}
		if _, err := r.db.NewInsert().Model(stack).Exec(ctx); err != nil {
			return nil, fmt.Errorf("add inventory item: %w", err)
		}
	default:
		return nil, fmt.Errorf("find inventory stack: %w", err)
	}
	return r.FindOrCreate(ctx, characterID)
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
