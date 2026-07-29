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
	"sshrpg/src/quest"
)

var (
	ErrQuestNotActive       = errors.New("quest is not active")
	ErrQuestItemsIncomplete = errors.New("quest items are incomplete")
)

type QuestCompletion struct {
	Quest     domain.CharacterQuest
	Inventory *domain.Inventory
	Gold      int
}

type AcceptQuestParams struct {
	CharacterID int64
	QuestID     string
	GiverID     string
}

type QuestRepository interface {
	FindByCharacter(context.Context, int64) ([]domain.CharacterQuest, error)
	Accept(context.Context, AcceptQuestParams) (domain.CharacterQuest, error)
	Complete(context.Context, int64, *quest.Definition) (QuestCompletion, error)
}

type BunQuestRepository struct {
	db bun.IDB
}

func NewQuestRepository(db bun.IDB) *BunQuestRepository {
	return &BunQuestRepository{db: db}
}

func (r *BunQuestRepository) FindByCharacter(
	ctx context.Context,
	characterID int64,
) ([]domain.CharacterQuest, error) {
	records := make([]entity.CharacterQuest, 0)
	if err := r.db.NewSelect().
		Model(&records).
		Where("character_id = ?", characterID).
		Order("accepted_at ASC", "quest_id ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("find character quests: %w", err)
	}
	quests := make([]domain.CharacterQuest, len(records))
	for index, record := range records {
		quests[index] = toDomainQuest(record)
	}
	return quests, nil
}

func (r *BunQuestRepository) Accept(
	ctx context.Context,
	params AcceptQuestParams,
) (domain.CharacterQuest, error) {
	record := &entity.CharacterQuest{
		CharacterID: params.CharacterID,
		QuestID:     params.QuestID,
		GiverID:     params.GiverID,
		Status:      string(domain.QuestActive),
		AcceptedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	result, err := r.db.NewInsert().Model(record).Ignore().Exec(ctx)
	if err != nil {
		return domain.CharacterQuest{}, fmt.Errorf("accept quest: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.CharacterQuest{}, fmt.Errorf("read accepted quest count: %w", err)
	}
	if affected == 0 {
		if err := r.db.NewSelect().
			Model(record).
			WherePK().
			Scan(ctx); err != nil {
			return domain.CharacterQuest{}, fmt.Errorf("find existing quest: %w", err)
		}
	}
	return toDomainQuest(*record), nil
}

func (r *BunQuestRepository) Complete(
	ctx context.Context,
	characterID int64,
	definition *quest.Definition,
) (QuestCompletion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return QuestCompletion{}, fmt.Errorf("begin quest completion: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	record := &entity.CharacterQuest{
		CharacterID: characterID,
		QuestID:     definition.ID,
	}
	err = tx.NewSelect().Model(record).WherePK().Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return QuestCompletion{}, ErrQuestNotActive
	}
	if err != nil {
		return QuestCompletion{}, fmt.Errorf("find active quest: %w", err)
	}
	if record.Status != string(domain.QuestActive) {
		return QuestCompletion{}, ErrQuestNotActive
	}

	stacks := make([]entity.InventoryItem, 0)
	if err := tx.NewSelect().
		Model(&stacks).
		Where("character_id = ?", characterID).
		Where("item_key = ?", definition.Objective.Item.ID).
		Order("slot ASC").
		Scan(ctx); err != nil {
		return QuestCompletion{}, fmt.Errorf("find quest items: %w", err)
	}
	total := 0
	for _, stack := range stacks {
		total += stack.Quantity
	}
	if total < definition.Objective.Quantity {
		return QuestCompletion{}, ErrQuestItemsIncomplete
	}

	remaining := definition.Objective.Quantity
	for index := range stacks {
		stack := &stacks[index]
		if remaining == 0 {
			break
		}
		if stack.Quantity <= remaining {
			remaining -= stack.Quantity
			if _, err := tx.NewDelete().Model(stack).WherePK().Exec(ctx); err != nil {
				return QuestCompletion{}, fmt.Errorf("consume quest item stack: %w", err)
			}
			continue
		}
		if _, err := tx.NewUpdate().
			Model(stack).
			Column("quantity").
			Set("quantity = quantity - ?", remaining).
			WherePK().
			Exec(ctx); err != nil {
			return QuestCompletion{}, fmt.Errorf("consume quest items: %w", err)
		}
		remaining = 0
	}

	if err := ensureCharacterProgress(ctx, tx, characterID); err != nil {
		return QuestCompletion{}, err
	}
	if definition.Reward.Gold > 0 {
		if _, err := tx.NewUpdate().
			Model((*entity.CharacterProgress)(nil)).
			Set("gold = gold + ?", definition.Reward.Gold).
			Where("character_id = ?", characterID).
			Exec(ctx); err != nil {
			return QuestCompletion{}, fmt.Errorf("add quest reward gold: %w", err)
		}
	}

	record.Status = string(domain.QuestCompleted)
	record.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.NewUpdate().
		Model(record).
		Column("status", "completed_at").
		WherePK().
		Where("status = ?", domain.QuestActive).
		Exec(ctx); err != nil {
		return QuestCompletion{}, fmt.Errorf("complete quest: %w", err)
	}
	inventory, err := (&BunInventoryRepository{db: tx}).FindOrCreate(ctx, characterID)
	if err != nil {
		return QuestCompletion{}, err
	}
	gold, err := characterGold(ctx, tx, characterID)
	if err != nil {
		return QuestCompletion{}, err
	}
	if err := tx.Commit(); err != nil {
		return QuestCompletion{}, fmt.Errorf("commit quest completion: %w", err)
	}
	return QuestCompletion{
		Quest: toDomainQuest(*record), Inventory: inventory, Gold: gold,
	}, nil
}

func toDomainQuest(record entity.CharacterQuest) domain.CharacterQuest {
	return domain.CharacterQuest{
		QuestID: record.QuestID, GiverID: record.GiverID,
		Status:     domain.QuestStatus(record.Status),
		AcceptedAt: record.AcceptedAt, CompletedAt: record.CompletedAt,
	}
}
