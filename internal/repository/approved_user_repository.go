package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// ApprovedUserRepository handles approved user database operations.
type ApprovedUserRepository struct {
	db database.PGXDB
}

// NewApprovedUserRepository creates a new ApprovedUserRepository.
func NewApprovedUserRepository(db database.PGXDB) *ApprovedUserRepository {
	return &ApprovedUserRepository{db: db}
}

// Approve adds a user by ID (and optional username) to the approved list.
func (r *ApprovedUserRepository) Approve(ctx context.Context, userID int64, username string, approvedBy int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO approved_users (user_id, username, approved_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) WHERE user_id != 0
		DO UPDATE SET username = EXCLUDED.username
	`, userID, username, approvedBy)
	if err != nil {
		return fmt.Errorf("failed to approve user: %w", err)
	}
	return nil
}

// ApproveByUsername adds a user by username only to the approved list.
func (r *ApprovedUserRepository) ApproveByUsername(ctx context.Context, username string, approvedBy int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO approved_users (user_id, username, approved_by)
		VALUES (0, $1, $2)
		ON CONFLICT (LOWER(username)) WHERE username != ''
		DO NOTHING
	`, username, approvedBy)
	if err != nil {
		return fmt.Errorf("failed to approve user by username: %w", err)
	}
	return nil
}

// IsApproved checks if a user is in the approved list by ID or username.
// The returned needsBackfill flag is true when the user was matched by
// username only (user_id is still 0), indicating UpdateUserID should be called.
func (r *ApprovedUserRepository) IsApproved(ctx context.Context, userID int64, username string) (approved bool, needsBackfill bool, err error) {
	var matchedUserID int64
	scanErr := r.db.QueryRow(ctx, `
		SELECT user_id FROM approved_users
		WHERE (user_id = $1 AND user_id != 0)
		   OR (LOWER(username) = LOWER($2) AND username != '')
		LIMIT 1
	`, userID, username).Scan(&matchedUserID)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to check approved status: %w", scanErr)
	}
	return true, matchedUserID == 0, nil
}

// Revoke removes a user from the approved list by user ID.
func (r *ApprovedUserRepository) Revoke(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM approved_users WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke user: %w", err)
	}
	return nil
}

// RevokeByUsername removes a user from the approved list by username.
func (r *ApprovedUserRepository) RevokeByUsername(ctx context.Context, username string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM approved_users WHERE LOWER(username) = LOWER($1)
	`, username)
	if err != nil {
		return fmt.Errorf("failed to revoke user by username: %w", err)
	}
	return nil
}

// UpdateUserID backfills the user_id for a username-only approved user.
func (r *ApprovedUserRepository) UpdateUserID(ctx context.Context, username string, userID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approved_users
		SET user_id = $1
		WHERE LOWER(username) = LOWER($2) AND user_id = 0
	`, userID, username)
	if err != nil {
		return fmt.Errorf("failed to update user ID: %w", err)
	}
	return nil
}

// GetAll returns all approved users.
func (r *ApprovedUserRepository) GetAll(ctx context.Context) ([]models.ApprovedUser, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, username, approved_by, created_at
		FROM approved_users
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get approved users: %w", err)
	}
	defer rows.Close()

	var users []models.ApprovedUser
	for rows.Next() {
		var u models.ApprovedUser
		if err := rows.Scan(&u.ID, &u.UserID, &u.Username, &u.ApprovedBy, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan approved user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate approved users: %w", err)
	}
	return users, nil
}
