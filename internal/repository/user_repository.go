package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// UserRepository handles user database operations.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// UpsertUser creates or updates a user.
func (r *UserRepository) UpsertUser(ctx context.Context, user *models.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, username, first_name, last_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			updated_at = NOW()
	`, user.ID, user.Username, user.FirstName, user.LastName)
	if err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by their Telegram ID.
func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, first_name, last_name, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Username, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}
