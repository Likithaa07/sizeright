// Package store saves and loads sizings in PostgreSQL.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct{ pool *pgxpool.Pool }

// Sizing is one saved row.
type Sizing struct {
	ID           int64           `json:"id"`
	CustomerID   string          `json:"customer_id"`
	Request      json.RawMessage `json:"request"`
	Result       json.RawMessage `json:"result"`
	RulesVersion string          `json:"rules_version"`
	CreatedBy    string          `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
}

// New opens a connection pool and checks the database is reachable.
func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Save inserts a sizing. $1, $2... are parameters: never build SQL with strings.
func (s *Store) Save(ctx context.Context, sz Sizing) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sizings (customer_id, request, result, rules_version, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		sz.CustomerID, sz.Request, sz.Result, sz.RulesVersion, sz.CreatedBy,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert sizing: %w", err)
	}
	return id, nil
}

// Get loads one sizing by ID.
func (s *Store) Get(ctx context.Context, id int64) (Sizing, error) {
	var sz Sizing
	err := s.pool.QueryRow(ctx,
		`SELECT id, customer_id, request, result, rules_version, created_by, created_at
		 FROM sizings WHERE id = $1`, id,
	).Scan(&sz.ID, &sz.CustomerID, &sz.Request, &sz.Result, &sz.RulesVersion, &sz.CreatedBy, &sz.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return sz, ErrNotFound
	}
	if err != nil {
		return sz, fmt.Errorf("get sizing: %w", err)
	}
	return sz, nil
}
