// Package store wraps a pgx pool with typed methods returning domain types.
//
// Conventions:
//   - SQL is inline in the file for the entity it concerns.
//   - All methods take a context as the first arg.
//   - Reads return (*T, error) for single rows ((nil, nil) for not-found),
//     or ([]T, error) for collections.
//   - Writes that need to span multiple statements use pool.BeginTx.
//   - Authorization (ownership) is the caller's responsibility — pass the
//     authenticated owner_id explicitly. The store never reads ambient state.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the underlying pool for handlers that need their own
// transaction (e.g. closing save).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// IsUniqueViolation returns true if err is a Postgres unique-constraint
// violation (SQLSTATE 23505). Handlers map this to a 409 / "duplicate".
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// IsNoRows returns true if the error reports "no rows in result set".
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
