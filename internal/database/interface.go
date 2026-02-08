package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGXDB is an interface that both pgxpool.Pool and pgx.Tx implement.
// This allows repositories to work with either a connection pool or a transaction,
// which is essential for testing with transaction-based isolation.
type PGXDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxBeginner can start a database transaction. Implemented by pgxpool.Pool.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Ensure types implement the interface at compile time.
var (
	_ PGXDB      = (*pgxpool.Pool)(nil)
	_ PGXDB      = (pgx.Tx)(nil)
	_ TxBeginner = (*pgxpool.Pool)(nil)
)
