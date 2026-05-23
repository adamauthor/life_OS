package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

func (r *CalendarActionRepository) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	type execer interface {
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}
	db, ok := r.db.(execer)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("db does not support Exec")
	}
	return db.Exec(ctx, sql, args...)
}
