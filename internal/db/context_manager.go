package db

import (
	"context"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ContextKey string

const (
	EngineKey ContextKey = "db.engine"
)

type ContextManager struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewContextManager(pool *pgxpool.Pool, logger *zap.Logger) *ContextManager {
	return &ContextManager{
		pool:   pool,
		logger: logger,
	}
}

type Engine interface {
	pgxscan.Querier
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}

type Transactioner interface {
	Do(ctx context.Context, f func(ctx context.Context) error) error
}

type EngineFactory interface {
	Get(ctx context.Context) Engine
}

func (cm *ContextManager) putEngineInContext(ctx context.Context, engine Engine) context.Context {
	return context.WithValue(ctx, EngineKey, engine)
}

func (cm *ContextManager) begin(ctx context.Context) (context.Context, error) {
	_, ok := ctx.Value(EngineKey).(pgx.Tx)
	if ok {
		return ctx, nil
	}

	tx, err := cm.pool.Begin(ctx)
	if err != nil {
		return ctx, err
	}

	return cm.putEngineInContext(ctx, tx), nil
}

func (cm *ContextManager) commit(ctx context.Context) error {
	tx, ok := ctx.Value(EngineKey).(pgx.Tx)
	if !ok {
		return nil
	}

	return tx.Commit(ctx)
}

func (cm *ContextManager) rollback(ctx context.Context) error {
	tx, ok := ctx.Value(EngineKey).(pgx.Tx)
	if !ok {
		return nil
	}

	return tx.Rollback(ctx)
}

func (cm *ContextManager) Do(ctx context.Context, f func(ctx context.Context) error) (err error) {
	txCtx, err := cm.begin(ctx)
	if err != nil {
		return err
	}

	detCtx := context.WithoutCancel(txCtx)
	defer func() {
		if p := recover(); p != nil {
			cm.logger.Error("panic occurred in transaction", zap.Any("panic", p))
			cm.rollback(detCtx)
			panic(p)
		}
		if err != nil {
			cm.logger.Error("error in transaction occurred", zap.Error(err))
			innerErr := cm.rollback(txCtx)
			if innerErr != nil {
				cm.logger.Error("failed to rollback transaction", zap.Error(innerErr))
			}
		} else {
			err = cm.commit(txCtx)
			if err != nil {
				cm.logger.Error("failed to commit transaction", zap.Error(err))
			}
		}
	}()

	err = f(txCtx)

	return err
}

func (cm *ContextManager) Get(ctx context.Context) Engine {
	if engine, ok := ctx.Value(EngineKey).(Engine); ok {
		return engine
	}
	return cm.pool
}
