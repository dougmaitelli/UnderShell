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
	AddItems(context.Context, int64, string, int, int) (*domain.Inventory, error)
	ConsumeItem(context.Context, int64, int, string) (*domain.Inventory, error)
	Equip(context.Context, int64, int, string, string) (*domain.Inventory, error)
	Unequip(context.Context, int64, string) (*domain.Inventory, error)
}

func (r *BunInventoryRepository) AddItem(
	ctx context.Context,
	characterID int64,
	itemKey string,
	maxStack int,
) (*domain.Inventory, error) {
	return r.AddItems(ctx, characterID, itemKey, maxStack, 1)
}

func (r *BunInventoryRepository) AddItems(
	ctx context.Context,
	characterID int64,
	itemKey string,
	maxStack int,
	quantity int,
) (*domain.Inventory, error) {
	if maxStack < 1 {
		return nil, errors.New("max stack must be at least 1")
	}
	if quantity < 1 {
		return nil, errors.New("quantity must be at least 1")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin add inventory items: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for quantity > 0 {
		stack := &entity.InventoryItem{}
		err := tx.NewSelect().
			Model(stack).
			Where("character_id = ?", characterID).
			Where("item_key = ?", itemKey).
			Where("quantity < ?", maxStack).
			Order("slot ASC").
			Limit(1).
			Scan(ctx)
		if err == nil {
			added := min(quantity, maxStack-stack.Quantity)
			if _, err := tx.NewUpdate().
				Model(stack).
				Column("quantity").
				Set("quantity = quantity + ?", added).
				WherePK().
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("increase inventory item: %w", err)
			}
			quantity -= added
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find inventory stack: %w", err)
		}
		var nextSlot int
		if err := tx.NewSelect().
			Model((*entity.InventoryItem)(nil)).
			ColumnExpr("COALESCE(MAX(slot), 0) + 1").
			Where("character_id = ?", characterID).
			Scan(ctx, &nextSlot); err != nil {
			return nil, fmt.Errorf("find next inventory slot: %w", err)
		}
		added := min(quantity, maxStack)
		stack = &entity.InventoryItem{
			CharacterID: characterID,
			Slot:        nextSlot, ItemKey: itemKey, Quantity: added,
		}
		if _, err := tx.NewInsert().Model(stack).Exec(ctx); err != nil {
			return nil, fmt.Errorf("add inventory item: %w", err)
		}
		quantity -= added
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit inventory items: %w", err)
	}
	return r.FindOrCreate(ctx, characterID)
}

type BunInventoryRepository struct {
	db bun.IDB
}

func NewInventoryRepository(db bun.IDB) *BunInventoryRepository {
	return &BunInventoryRepository{db: db}
}

func (r *BunInventoryRepository) ConsumeItem(
	ctx context.Context,
	characterID int64,
	slot int,
	itemKey string,
) (*domain.Inventory, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin consume inventory item: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	stack := new(entity.InventoryItem)
	err = tx.NewSelect().
		Model(stack).
		Where("character_id = ?", characterID).
		Where("slot = ?", slot).
		Where("item_key = ?", itemKey).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrItemNotOwned
	}
	if err != nil {
		return nil, fmt.Errorf("find consumed inventory item: %w", err)
	}
	if stack.Quantity > 1 {
		if _, err := tx.NewUpdate().
			Model(stack).
			Column("quantity").
			Set("quantity = quantity - 1").
			WherePK().
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("decrease consumed inventory item: %w", err)
		}
	} else if _, err := tx.NewDelete().Model(stack).WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("remove consumed inventory item: %w", err)
	}
	inventory, err := (&BunInventoryRepository{db: tx}).
		FindOrCreate(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consumed inventory item: %w", err)
	}
	return inventory, nil
}

func (r *BunInventoryRepository) Equip(
	ctx context.Context,
	characterID int64,
	inventorySlot int,
	itemKey string,
	equipmentSlot string,
) (*domain.Inventory, error) {
	if equipmentSlot == "" {
		return nil, errors.New("equipment slot is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin equip item: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var count int
	if err := tx.NewSelect().
		Model((*entity.InventoryItem)(nil)).
		ColumnExpr("COUNT(*)").
		Where("character_id = ?", characterID).
		Where("slot = ?", inventorySlot).
		Where("item_key = ?", itemKey).
		Scan(ctx, &count); err != nil {
		return nil, fmt.Errorf("find equipped inventory item: %w", err)
	}
	if count == 0 {
		return nil, ErrItemNotOwned
	}
	if _, err := tx.NewDelete().
		Model((*entity.CharacterEquipment)(nil)).
		Where("character_id = ?", characterID).
		Where("inventory_slot = ?", inventorySlot).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("clear previous item assignment: %w", err)
	}
	assignment := &entity.CharacterEquipment{
		CharacterID: characterID, EquipmentSlot: equipmentSlot,
		InventorySlot: inventorySlot,
	}
	if _, err := tx.NewInsert().
		Model(assignment).
		On("CONFLICT (character_id, equipment_slot) DO UPDATE").
		Set("inventory_slot = EXCLUDED.inventory_slot").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("equip inventory item: %w", err)
	}
	inventory, err := (&BunInventoryRepository{db: tx}).
		FindOrCreate(ctx, characterID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit equipped item: %w", err)
	}
	return inventory, nil
}

func (r *BunInventoryRepository) Unequip(
	ctx context.Context,
	characterID int64,
	equipmentSlot string,
) (*domain.Inventory, error) {
	if _, err := r.db.NewDelete().
		Model((*entity.CharacterEquipment)(nil)).
		Where("character_id = ?", characterID).
		Where("equipment_slot = ?", equipmentSlot).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("unequip inventory item: %w", err)
	}
	return r.FindOrCreate(ctx, characterID)
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
	equipment := make([]entity.CharacterEquipment, 0)
	if err := r.db.NewSelect().
		Model(&equipment).
		Where("character_id = ?", characterID).
		Order("equipment_slot ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("find equipped items: %w", err)
	}

	inventory := &domain.Inventory{
		CharacterID: characterID,
		Items:       make([]domain.InventoryItem, len(items)),
		Equipment:   make([]domain.EquippedItem, len(equipment)),
	}
	for index, item := range items {
		inventory.Items[index] = domain.InventoryItem{
			Slot: item.Slot, ItemKey: item.ItemKey, Quantity: item.Quantity,
		}
	}
	for index, equipped := range equipment {
		inventory.Equipment[index] = domain.EquippedItem{
			EquipmentSlot: equipped.EquipmentSlot,
			InventorySlot: equipped.InventorySlot,
		}
	}
	return inventory, nil
}
