package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations creates the database schema.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS categories (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS expenses (
			id SERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id),
			amount DECIMAL(12, 2) NOT NULL,
			currency TEXT NOT NULL DEFAULT 'SGD',
			description TEXT,
			category_id INTEGER REFERENCES categories(id),
			receipt_file_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_expenses_user_id ON expenses(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_created_at ON expenses(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_category_id ON expenses(category_id)`,

		`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'confirmed'`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_status ON expenses(status)`,

		`ALTER TABLE users ADD COLUMN IF NOT EXISTS default_currency TEXT NOT NULL DEFAULT 'SGD'`,
	}

	for i, migration := range migrations {
		if _, err := pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	return nil
}

// SeedCategories inserts the default expense categories.
func SeedCategories(ctx context.Context, pool *pgxpool.Pool) error {
	categories := []string{
		"Food - Dining Out",
		"Food - Grocery",
		"Transportation",
		"Communication",
		"Housing - Mortgage",
		"Housing - Others",
		"Personal Care",
		"Health and Wellness",
		"Education",
		"Entertainment",
		"Credit/Debt Payments",
		"Others",
		"Utilities",
		"Travel & Vacation",
		"Subscriptions",
		"Donations",
	}

	for _, cat := range categories {
		_, err := pool.Exec(ctx,
			`INSERT INTO categories (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`,
			cat,
		)
		if err != nil {
			return fmt.Errorf("failed to seed category %q: %w", cat, err)
		}
	}

	return nil
}
