package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"sshrpg/src/domain"
	"sshrpg/src/persistence/entity"
)

var (
	ErrInsufficientGold = errors.New("not enough gold")
	ErrItemNotOwned     = errors.New("item is no longer in the inventory")
	ErrItemEquipped     = errors.New("equipped items cannot be sold")
)

type TradeResult struct {
	Inventory *domain.Inventory
	Gold      int
}

type ShopRepository interface {
	BuyItem(context.Context, int64, string, int, int) (TradeResult, error)
	SellItem(context.Context, int64, int, string, int) (TradeResult, error)
}

type BunShopRepository struct {
	db bun.IDB
}

func NewShopRepository(db bun.IDB) *BunShopRepository {
	return &BunShopRepository{db: db}
}

func (r *BunShopRepository) BuyItem(
	ctx context.Context,
	characterID int64,
	itemKey string,
	maxStack int,
	price int,
) (TradeResult, error) {
	if price < 1 {
		return TradeResult{}, errors.New("buy price must be positive")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TradeResult{}, fmt.Errorf("begin shop purchase: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := ensureCharacterProgress(ctx, tx, characterID); err != nil {
		return TradeResult{}, err
	}
	result, err := tx.NewUpdate().
		Model((*entity.CharacterProgress)(nil)).
		Set("gold = gold - ?", price).
		Where("character_id = ?", characterID).
		Where("gold >= ?", price).
		Exec(ctx)
	if err != nil {
		return TradeResult{}, fmt.Errorf("spend gold: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return TradeResult{}, fmt.Errorf("read spent gold count: %w", err)
	}
	if affected == 0 {
		return TradeResult{}, ErrInsufficientGold
	}
	inventory, err := (&BunInventoryRepository{db: tx}).AddItem(
		ctx, characterID, itemKey, maxStack,
	)
	if err != nil {
		return TradeResult{}, err
	}
	gold, err := characterGold(ctx, tx, characterID)
	if err != nil {
		return TradeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return TradeResult{}, fmt.Errorf("commit shop purchase: %w", err)
	}
	return TradeResult{Inventory: inventory, Gold: gold}, nil
}

func (r *BunShopRepository) SellItem(
	ctx context.Context,
	characterID int64,
	slot int,
	itemKey string,
	price int,
) (TradeResult, error) {
	if price < 1 {
		return TradeResult{}, errors.New("sell price must be positive")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TradeResult{}, fmt.Errorf("begin shop sale: %w", err)
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
		return TradeResult{}, ErrItemNotOwned
	}
	if err != nil {
		return TradeResult{}, fmt.Errorf("find sold inventory item: %w", err)
	}
	var equippedCount int
	if err := tx.NewSelect().
		Model((*entity.CharacterEquipment)(nil)).
		ColumnExpr("COUNT(*)").
		Where("character_id = ?", characterID).
		Where("inventory_slot = ?", slot).
		Scan(ctx, &equippedCount); err != nil {
		return TradeResult{}, fmt.Errorf("check sold equipment: %w", err)
	}
	if equippedCount > 0 {
		return TradeResult{}, ErrItemEquipped
	}
	if stack.Quantity > 1 {
		if _, err := tx.NewUpdate().
			Model(stack).
			Column("quantity").
			Set("quantity = quantity - 1").
			WherePK().
			Exec(ctx); err != nil {
			return TradeResult{}, fmt.Errorf("decrease sold inventory item: %w", err)
		}
	} else {
		if _, err := tx.NewDelete().Model(stack).WherePK().Exec(ctx); err != nil {
			return TradeResult{}, fmt.Errorf("remove sold inventory item: %w", err)
		}
	}
	if err := ensureCharacterProgress(ctx, tx, characterID); err != nil {
		return TradeResult{}, err
	}
	if _, err := tx.NewUpdate().
		Model((*entity.CharacterProgress)(nil)).
		Set("gold = gold + ?", price).
		Where("character_id = ?", characterID).
		Exec(ctx); err != nil {
		return TradeResult{}, fmt.Errorf("add sale gold: %w", err)
	}
	inventory, err := (&BunInventoryRepository{db: tx}).FindOrCreate(ctx, characterID)
	if err != nil {
		return TradeResult{}, err
	}
	gold, err := characterGold(ctx, tx, characterID)
	if err != nil {
		return TradeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return TradeResult{}, fmt.Errorf("commit shop sale: %w", err)
	}
	return TradeResult{Inventory: inventory, Gold: gold}, nil
}

func ensureCharacterProgress(ctx context.Context, db bun.IDB, characterID int64) error {
	progress := &entity.CharacterProgress{
		CharacterID: characterID,
		Level:       1,
		Gold:        domain.DefaultStartingGold,
	}
	if _, err := db.NewInsert().Model(progress).Ignore().Exec(ctx); err != nil {
		return fmt.Errorf("initialize character progress: %w", err)
	}
	return nil
}

func characterGold(ctx context.Context, db bun.IDB, characterID int64) (int, error) {
	var gold int
	if err := db.NewSelect().
		Model((*entity.CharacterProgress)(nil)).
		Column("gold").
		Where("character_id = ?", characterID).
		Scan(ctx, &gold); err != nil {
		return 0, fmt.Errorf("load character gold: %w", err)
	}
	return gold, nil
}
