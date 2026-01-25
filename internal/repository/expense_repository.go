package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// ExpenseRepository handles expense database operations.
type ExpenseRepository struct {
	pool *pgxpool.Pool
}

// NewExpenseRepository creates a new ExpenseRepository.
func NewExpenseRepository(pool *pgxpool.Pool) *ExpenseRepository {
	return &ExpenseRepository{pool: pool}
}

// Create adds a new expense.
func (r *ExpenseRepository) Create(ctx context.Context, expense *models.Expense) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO expenses (user_id, amount, currency, description, category_id, receipt_file_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, expense.UserID, expense.Amount, expense.Currency, expense.Description,
		expense.CategoryID, expense.ReceiptFileID,
	).Scan(&expense.ID, &expense.CreatedAt, &expense.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create expense: %w", err)
	}
	return nil
}

// GetByID retrieves an expense by ID.
func (r *ExpenseRepository) GetByID(ctx context.Context, id int) (*models.Expense, error) {
	var exp models.Expense
	var categoryID *int
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, amount, currency, description, category_id, receipt_file_id, created_at, updated_at
		FROM expenses WHERE id = $1
	`, id).Scan(&exp.ID, &exp.UserID, &exp.Amount, &exp.Currency, &exp.Description,
		&categoryID, &exp.ReceiptFileID, &exp.CreatedAt, &exp.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	exp.CategoryID = categoryID
	return &exp, nil
}

// GetByUserID retrieves all expenses for a user.
func (r *ExpenseRepository) GetByUserID(ctx context.Context, userID int64, limit int) ([]models.Expense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.user_id, e.amount, e.currency, e.description, e.category_id,
		       e.receipt_file_id, e.created_at, e.updated_at,
		       c.id, c.name, c.created_at
		FROM expenses e
		LEFT JOIN categories c ON e.category_id = c.id
		WHERE e.user_id = $1
		ORDER BY e.created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}
	defer rows.Close()

	return scanExpenses(rows)
}

// GetByUserIDAndDateRange retrieves expenses for a user within a date range.
func (r *ExpenseRepository) GetByUserIDAndDateRange(
	ctx context.Context,
	userID int64,
	startDate, endDate time.Time,
) ([]models.Expense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.user_id, e.amount, e.currency, e.description, e.category_id,
		       e.receipt_file_id, e.created_at, e.updated_at,
		       c.id, c.name, c.created_at
		FROM expenses e
		LEFT JOIN categories c ON e.category_id = c.id
		WHERE e.user_id = $1 AND e.created_at >= $2 AND e.created_at < $3
		ORDER BY e.created_at DESC
	`, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses by date range: %w", err)
	}
	defer rows.Close()

	return scanExpenses(rows)
}

// Update modifies an existing expense.
func (r *ExpenseRepository) Update(ctx context.Context, expense *models.Expense) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE expenses SET
			amount = $2,
			currency = $3,
			description = $4,
			category_id = $5,
			receipt_file_id = $6,
			updated_at = NOW()
		WHERE id = $1
	`, expense.ID, expense.Amount, expense.Currency, expense.Description,
		expense.CategoryID, expense.ReceiptFileID)
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}
	return nil
}

// Delete removes an expense by ID.
func (r *ExpenseRepository) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM expenses WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete expense: %w", err)
	}
	return nil
}

// GetTotalByUserIDAndDateRange calculates total spending for a user in a date range.
func (r *ExpenseRepository) GetTotalByUserIDAndDateRange(
	ctx context.Context,
	userID int64,
	startDate, endDate time.Time,
) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM expenses
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`, userID, startDate, endDate).Scan(&total)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get total: %w", err)
	}
	return total, nil
}

// scanExpenses is a helper to scan expense rows with category joins.
func scanExpenses(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]models.Expense, error) {
	var expenses []models.Expense
	for rows.Next() {
		var exp models.Expense
		var categoryID, catID *int
		var catName *string
		var catCreatedAt *time.Time

		if err := rows.Scan(
			&exp.ID, &exp.UserID, &exp.Amount, &exp.Currency, &exp.Description,
			&categoryID, &exp.ReceiptFileID, &exp.CreatedAt, &exp.UpdatedAt,
			&catID, &catName, &catCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expense: %w", err)
		}

		exp.CategoryID = categoryID
		if catID != nil {
			exp.Category = &models.Category{
				ID:        *catID,
				Name:      *catName,
				CreatedAt: *catCreatedAt,
			}
		}
		expenses = append(expenses, exp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating expenses: %w", err)
	}
	return expenses, nil
}
