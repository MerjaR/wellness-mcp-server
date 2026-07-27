// Package db wraps the pgx connection pool used to read recipe data.
// Adapted from the wellness-platform's database package — this server
// only ever reads, so no write-tuning is needed here.
package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the pgx connection pool.
type DB struct {
	*pgxpool.Pool
}

// New creates a new PostgreSQL connection pool using the provided DSN.
func New(ctx context.Context, dsn string) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database DSN: %w", err)
	}

	// This server is read-only and low-traffic, so a smaller pool than
	// the main app is plenty.
	config.MaxConns = 5
	config.MinConns = 1
	config.MaxConnLifetime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL database")
	return &DB{pool}, nil
}

// Close closes the database connection pool if it exists.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}