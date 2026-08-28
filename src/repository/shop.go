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
	var trade TradeResult
	err := runEconomicTransaction(ctx, r.db, "shop purchase", func(tx bun.Tx) error {
		if err := lockInventory(ctx, tx, characterID); err != nil {
			return err
		}
		if err := ensureCharacterProgress(ctx, tx, characterID); err != nil {
			return err
		}
		if err := lockCharacterProgress(ctx, tx, characterID); err != nil {
			return err
		}
		result, err := tx.NewUpdate().
			Model((*entity.CharacterProgress)(nil)).
			Set("gold = gold - ?", price).
			Where("character_id = ?", characterID).
			Where("gold >= ?", price).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("spend gold: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read spent gold count: %w", err)
		}
		if affected == 0 {
			return ErrInsufficientGold
		}
		inventory, err := addItems(ctx, tx, characterID, itemKey, maxStack, 1)
		if err != nil {
			return err
		}
		gold, err := characterGold(ctx, tx, characterID)
		if err != nil {
			return err
		}
		trade = TradeResult{Inventory: inventory, Gold: gold}
		return nil
	})
	if err != nil {
		return TradeResult{}, err
	}
	return trade, nil
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
	var trade TradeResult
	err := runEconomicTransaction(ctx, r.db, "shop sale", func(tx bun.Tx) error {
		if err := lockInventory(ctx, tx, characterID); err != nil {
			return err
		}
		stack := new(entity.InventoryItem)
		err := lockForUpdate(tx.NewSelect().
			Model(stack).
			Where("character_id = ?", characterID).
			Where("slot = ?", slot).
			Where("item_key = ?", itemKey), tx).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrItemNotOwned
		}
		if err != nil {
			return fmt.Errorf("find sold inventory item: %w", err)
		}
		var equippedCount int
		if err := tx.NewSelect().
			Model((*entity.CharacterEquipment)(nil)).
			ColumnExpr("COUNT(*)").
			Where("character_id = ?", characterID).
			Where("inventory_slot = ?", slot).
			Scan(ctx, &equippedCount); err != nil {
			return fmt.Errorf("check sold equipment: %w", err)
		}
		if equippedCount > 0 {
			return ErrItemEquipped
		}
		var mutation sql.Result
		if stack.Quantity > 1 {
			mutation, err = tx.NewUpdate().
				Model(stack).
				Column("quantity").
				Set("quantity = quantity - 1").
				WherePK().Where("quantity = ?", stack.Quantity).Exec(ctx)
		} else {
			mutation, err = tx.NewDelete().Model(stack).WherePK().
				Where("quantity = 1").Exec(ctx)
		}
		if err != nil {
			return fmt.Errorf("remove sold inventory item: %w", err)
		}
		if err := requireOneRow(mutation, "sell inventory item"); err != nil {
			return err
		}
		if err := ensureCharacterProgress(ctx, tx, characterID); err != nil {
			return err
		}
		if err := lockCharacterProgress(ctx, tx, characterID); err != nil {
			return err
		}
		result, err := tx.NewUpdate().
			Model((*entity.CharacterProgress)(nil)).
			Set("gold = gold + ?", price).
			Where("character_id = ?", characterID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("add sale gold: %w", err)
		}
		if err := requireOneRow(result, "add sale gold"); err != nil {
			return err
		}
		inventory, err := (&BunInventoryRepository{db: tx}).FindOrCreate(ctx, characterID)
		if err != nil {
			return err
		}
		gold, err := characterGold(ctx, tx, characterID)
		if err != nil {
			return err
		}
		trade = TradeResult{Inventory: inventory, Gold: gold}
		return nil
	})
	if err != nil {
		return TradeResult{}, err
	}
	return trade, nil
}

func lockCharacterProgress(ctx context.Context, db bun.IDB, characterID int64) error {
	progress := new(entity.CharacterProgress)
	if err := lockForUpdate(db.NewSelect().Model(progress).
		Where("character_id = ?", characterID), db).Scan(ctx); err != nil {
		return fmt.Errorf("lock character progress: %w", err)
	}
	return nil
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
