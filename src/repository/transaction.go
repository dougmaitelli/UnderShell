package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const economicTransactionAttempts = 3

// runEconomicTransaction retries only PostgreSQL serialization failures and
// deadlocks. The whole transaction is replayed; individual statements are
// never retried after their transaction has been aborted.
func runEconomicTransaction(
	ctx context.Context,
	db bun.IDB,
	operation string,
	fn func(bun.Tx) error,
) error {
	for attempt := 1; ; attempt++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin %s: %w", operation, err)
		}
		err = fn(tx)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			_ = tx.Rollback()
			if attempt < economicTransactionAttempts && isRetryableTransactionError(err) {
				continue
			}
			return err
		}
		return nil
	}
}

func lockForUpdate(query *bun.SelectQuery, db bun.IDB) *bun.SelectQuery {
	if db.Dialect().Name() == dialect.PG {
		return query.For("UPDATE")
	}
	return query
}

func requireOneRow(result interface{ RowsAffected() (int64, error) }, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s row count: %w", operation, err)
	}
	if affected != 1 {
		return fmt.Errorf("%s: concurrent state change", operation)
	}
	return nil
}

func isRetryableTransactionError(err error) bool {
	var postgresError interface{ Field(byte) string }
	if !errors.As(err, &postgresError) {
		return false
	}
	switch postgresError.Field('C') {
	case "40001", "40P01":
		return true
	default:
		return false
	}
}
