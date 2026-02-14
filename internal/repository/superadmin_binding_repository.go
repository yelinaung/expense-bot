package repository

import (
	"context"
	"fmt"

	"gitlab.com/yelinaung/expense-bot/internal/database"
)

// SuperadminBinding represents a persisted username → user_id binding.
type SuperadminBinding struct {
	Username string
	UserID   int64
}

// SuperadminBindingRepository handles persisted superadmin bindings.
type SuperadminBindingRepository struct {
	db database.PGXDB
}

// NewSuperadminBindingRepository creates a new SuperadminBindingRepository.
func NewSuperadminBindingRepository(db database.PGXDB) *SuperadminBindingRepository {
	return &SuperadminBindingRepository{db: db}
}

// LoadAll returns all persisted superadmin bindings.
func (r *SuperadminBindingRepository) LoadAll(ctx context.Context) ([]SuperadminBinding, error) {
	rows, err := r.db.Query(ctx, `SELECT username, user_id FROM superadmin_bindings`)
	if err != nil {
		return nil, fmt.Errorf("failed to load superadmin bindings: %w", err)
	}
	defer rows.Close()

	var bindings []SuperadminBinding
	for rows.Next() {
		var b SuperadminBinding
		if err := rows.Scan(&b.Username, &b.UserID); err != nil {
			return nil, fmt.Errorf("failed to scan superadmin binding: %w", err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate superadmin bindings: %w", err)
	}
	return bindings, nil
}

// Save persists a username → user_id binding. Uses upsert so it is
// safe to call multiple times with the same username.
func (r *SuperadminBindingRepository) Save(ctx context.Context, username string, userID int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO superadmin_bindings (username, user_id)
		VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE SET user_id = EXCLUDED.user_id
	`, username, userID)
	if err != nil {
		return fmt.Errorf("failed to save superadmin binding: %w", err)
	}
	return nil
}
