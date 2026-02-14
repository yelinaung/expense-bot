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

		`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS user_expense_number BIGINT`,

		`CREATE TABLE IF NOT EXISTS user_expense_counters (
			user_id BIGINT PRIMARY KEY REFERENCES users(id),
			next_number BIGINT NOT NULL DEFAULT 1
		)`,

		`WITH numbered AS (
			SELECT id,
			       row_number() OVER (PARTITION BY user_id ORDER BY created_at, id) AS rn
			FROM expenses
			WHERE user_expense_number IS NULL
		)
		UPDATE expenses e
		SET user_expense_number = n.rn
		FROM numbered n
		WHERE e.id = n.id`,

		`INSERT INTO user_expense_counters (user_id, next_number)
		SELECT user_id, COALESCE(MAX(user_expense_number), 0) + 1
		FROM expenses
		GROUP BY user_id
		ON CONFLICT (user_id)
		DO UPDATE SET next_number = GREATEST(user_expense_counters.next_number, EXCLUDED.next_number)`,

		`CREATE OR REPLACE FUNCTION set_user_expense_number()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$
		DECLARE v BIGINT;
		BEGIN
			IF NEW.user_expense_number IS NOT NULL THEN
				RETURN NEW;
			END IF;

			INSERT INTO user_expense_counters (user_id, next_number)
			VALUES (NEW.user_id, 2)
			ON CONFLICT (user_id)
			DO UPDATE SET next_number = user_expense_counters.next_number + 1
			RETURNING next_number - 1 INTO v;

			NEW.user_expense_number := v;
			RETURN NEW;
		END;
		$$`,

		`DROP TRIGGER IF EXISTS trg_set_user_expense_number ON expenses`,

		`CREATE TRIGGER trg_set_user_expense_number
		BEFORE INSERT ON expenses
		FOR EACH ROW
		EXECUTE FUNCTION set_user_expense_number()`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_expenses_user_number
		ON expenses(user_id, user_expense_number)`,

		`ALTER TABLE expenses ADD COLUMN IF NOT EXISTS merchant TEXT NOT NULL DEFAULT ''`,

		`CREATE TABLE IF NOT EXISTS tags (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS expense_tags (
			expense_id INTEGER NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (expense_id, tag_id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_expense_tags_tag_id ON expense_tags(tag_id)`,

		`CREATE TABLE IF NOT EXISTS approved_users (
			id SERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL DEFAULT 0,
			username TEXT NOT NULL DEFAULT '',
			approved_by BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_approved_users_user_id
			ON approved_users(user_id) WHERE user_id != 0`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_approved_users_username
			ON approved_users(LOWER(username)) WHERE username != ''`,

		`CREATE TABLE IF NOT EXISTS superadmin_bindings (
			username TEXT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
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
