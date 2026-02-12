package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// UserRepository handles user database operations.
type UserRepository struct {
	db database.PGXDB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db database.PGXDB) *UserRepository {
	return &UserRepository{db: db}
}

// UpsertUser creates or updates a user.
func (r *UserRepository) UpsertUser(ctx context.Context, user *models.User) error {
	_, err := r.db.Exec(ctx, `
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
	err := r.db.QueryRow(ctx, `
		SELECT id, username, first_name, last_name, default_currency, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&user.ID, &user.Username, &user.FirstName, &user.LastName, &user.DefaultCurrency, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// UpdateDefaultCurrency updates a user's default currency.
func (r *UserRepository) UpdateDefaultCurrency(ctx context.Context, userID int64, currency string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET default_currency = $2, updated_at = NOW() WHERE id = $1
	`, userID, currency)
	if err != nil {
		return fmt.Errorf("failed to update default currency: %w", err)
	}
	return nil
}

// GetAllUsers returns all registered users.
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, username, first_name, last_name, default_currency, created_at, updated_at
		FROM users
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &u.DefaultCurrency, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}
	return users, nil
}

// GetUsersNeedingReminder returns authorized users who have no confirmed expenses
// in the given time range. Authorization means the user is either a superadmin
// (by ID or username) or exists in the approved_users table.
func (r *UserRepository) GetUsersNeedingReminder(
	ctx context.Context,
	superAdminIDs []int64,
	superAdminUsernames []string,
	startOfDay, endOfDay time.Time,
) ([]models.User, error) {
	lowered := make([]string, len(superAdminUsernames))
	for i, u := range superAdminUsernames {
		lowered[i] = strings.ToLower(u)
	}

	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.username, u.first_name, u.last_name
		FROM users u
		WHERE (
			u.id = ANY($1)
			OR LOWER(u.username) = ANY($2::text[])
			OR EXISTS (SELECT 1 FROM approved_users au WHERE au.user_id = u.id AND au.user_id != 0)
			OR EXISTS (SELECT 1 FROM approved_users au WHERE LOWER(au.username) = LOWER(u.username) AND u.username != '' AND au.username != '')
		)
		AND NOT EXISTS (
			SELECT 1 FROM expenses e
			WHERE e.user_id = u.id AND e.created_at >= $3 AND e.created_at < $4 AND e.status = 'confirmed'
		)
	`, superAdminIDs, lowered, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("failed to query users needing reminder: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}
	return users, nil
}

// GetDefaultCurrency returns a user's default currency, or SGD if not set.
func (r *UserRepository) GetDefaultCurrency(ctx context.Context, userID int64) (string, error) {
	var currency string
	err := r.db.QueryRow(ctx, `
		SELECT default_currency FROM users WHERE id = $1
	`, userID).Scan(&currency)
	if err != nil {
		return models.DefaultCurrency, fmt.Errorf("failed to get default currency: %w", err)
	}
	if currency == "" {
		return models.DefaultCurrency, nil
	}
	return currency, nil
}
