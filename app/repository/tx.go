package repository

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TxManager membuka transaction dan mengulangnya saat terjadi deadlock
// atau serialization failure.
type TxManager struct {
	pool        *pgxpool.Pool
	maxAttempts int
}

// NewTxManager membuat pengelola transaction dengan maksimal tiga percobaan.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool, maxAttempts: 3}
}

// WithTx menjalankan callback dalam transaction dan mengulang deadlock
// atau serialization failure.
func (m *TxManager) WithTx(ctx context.Context, fn func(tx DBTX) error) error {
	var err error
	for attempt := 1; attempt <= m.maxAttempts; attempt++ {
		err = m.runOnce(ctx, fn)
		if !isRetryable(err) {
			return err
		}

		wait := time.Duration(attempt*20+rand.IntN(30)) * time.Millisecond
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
	}
	return err
}

// runOnce menjalankan callback dalam satu transaction.
func (m *TxManager) runOnce(ctx context.Context, fn func(tx DBTX) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("memulai transaction: %w", err)
	}

	// Baris rollback otomatis yang hilang merupakan bug puzzle
	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// isRetryable memeriksa apakah error berasal dari deadlock atau serialization failure.
func isRetryable(err error) bool {
	code := pgErrorCode(err)
	return code == codeSerializationFailure || code == codeDeadlockDetected
}
