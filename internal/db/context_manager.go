package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// contextKey is a custom type for context keys to avoid collisions
// Using UUID to ensure uniqueness
type contextKey struct {
	name string
}

var txKey = contextKey{name: uuid.New().String()}

// ContextManager manages database transactions
type ContextManager struct {
	pool *pgxpool.Pool
}

// NewContextManager creates a new context manager
func NewContextManager(pool *pgxpool.Pool) *ContextManager {
	return &ContextManager{pool: pool}
}

// GetQueryable returns either a transaction or the pool connection
func (cm *ContextManager) GetQueryable(ctx context.Context) Queryable {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return tx
	}
	return cm.pool
}

// Do executes a function within a transaction
func (cm *ContextManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	// If already in a transaction, just execute the function
	if _, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return fn(ctx)
	}

	// Start a new transaction
	tx, err := cm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Add transaction to context
	ctx = context.WithValue(ctx, txKey, tx)

	// Execute function
	err = fn(ctx)
	if err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Queryable interface for both pgxpool.Pool and pgx.Tx
type Queryable interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgx.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}
