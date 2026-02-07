package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// ExpenseRepository handles expense database operations.
type ExpenseRepository struct {
	db database.PGXDB
}

// NewExpenseRepository creates a new ExpenseRepository.
func NewExpenseRepository(db database.PGXDB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

// Pool returns the underlying database pool. Used for testing.
func (r *ExpenseRepository) Pool() database.PGXDB {
	return r.db
}

// Create adds a new expense.
func (r *ExpenseRepository) Create(ctx context.Context, expense *models.Expense) error {
	// Default to confirmed if not specified.
	if expense.Status == "" {
		expense.Status = models.ExpenseStatusConfirmed
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO expenses (user_id, amount, currency, description, merchant, category_id, receipt_file_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_expense_number, created_at, updated_at
	`, expense.UserID, expense.Amount, expense.Currency, expense.Description,
		expense.Merchant, expense.CategoryID, expense.ReceiptFileID, expense.Status,
	).Scan(&expense.ID, &expense.UserExpenseNumber, &expense.CreatedAt, &expense.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create expense: %w", err)
	}
	return nil
}

// GetByID retrieves an expense by ID.
func (r *ExpenseRepository) GetByID(ctx context.Context, id int) (*models.Expense, error) {
	var exp models.Expense
	var categoryID *int
	err := r.db.QueryRow(ctx, `
		SELECT id, user_expense_number, user_id, amount, currency, description, merchant, category_id, receipt_file_id, status, created_at, updated_at
		FROM expenses WHERE id = $1
	`, id).Scan(&exp.ID, &exp.UserExpenseNumber, &exp.UserID, &exp.Amount, &exp.Currency, &exp.Description,
		&exp.Merchant, &categoryID, &exp.ReceiptFileID, &exp.Status, &exp.CreatedAt, &exp.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense: %w", err)
	}
	exp.CategoryID = categoryID
	return &exp, nil
}

// GetByUserAndNumber retrieves an expense by user ID and per-user expense number.
func (r *ExpenseRepository) GetByUserAndNumber(ctx context.Context, userID int64, number int64) (*models.Expense, error) {
	var exp models.Expense
	var categoryID *int
	err := r.db.QueryRow(ctx, `
		SELECT id, user_expense_number, user_id, amount, currency, description, merchant, category_id, receipt_file_id, status, created_at, updated_at
		FROM expenses WHERE user_id = $1 AND user_expense_number = $2
	`, userID, number).Scan(&exp.ID, &exp.UserExpenseNumber, &exp.UserID, &exp.Amount, &exp.Currency, &exp.Description,
		&exp.Merchant, &categoryID, &exp.ReceiptFileID, &exp.Status, &exp.CreatedAt, &exp.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense by user number: %w", err)
	}
	exp.CategoryID = categoryID
	return &exp, nil
}

// GetByUserID retrieves all confirmed expenses for a user.
func (r *ExpenseRepository) GetByUserID(ctx context.Context, userID int64, limit int) ([]models.Expense, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.user_expense_number, e.user_id, e.amount, e.currency, e.description, e.merchant, e.category_id,
		       e.receipt_file_id, e.status, e.created_at, e.updated_at,
		       c.id, c.name, c.created_at
		FROM expenses e
		LEFT JOIN categories c ON e.category_id = c.id
		WHERE e.user_id = $1 AND e.status = 'confirmed'
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}
	defer rows.Close()

	return scanExpenses(rows)
}

// GetByUserIDAndDateRange retrieves confirmed expenses for a user within a date range.
func (r *ExpenseRepository) GetByUserIDAndDateRange(
	ctx context.Context,
	userID int64,
	startDate, endDate time.Time,
) ([]models.Expense, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.user_expense_number, e.user_id, e.amount, e.currency, e.description, e.merchant, e.category_id,
		       e.receipt_file_id, e.status, e.created_at, e.updated_at,
		       c.id, c.name, c.created_at
		FROM expenses e
		LEFT JOIN categories c ON e.category_id = c.id
		WHERE e.user_id = $1 AND e.created_at >= $2 AND e.created_at < $3 AND e.status = 'confirmed'
		ORDER BY e.created_at DESC, e.id DESC
	`, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses by date range: %w", err)
	}
	defer rows.Close()

	return scanExpenses(rows)
}

// GetByUserIDAndCategory retrieves confirmed expenses for a user filtered by category.
func (r *ExpenseRepository) GetByUserIDAndCategory(
	ctx context.Context,
	userID int64,
	categoryID int,
	limit int,
) ([]models.Expense, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.user_expense_number, e.user_id, e.amount, e.currency, e.description, e.merchant, e.category_id,
		       e.receipt_file_id, e.status, e.created_at, e.updated_at,
		       c.id, c.name, c.created_at
		FROM expenses e
		LEFT JOIN categories c ON e.category_id = c.id
		WHERE e.user_id = $1 AND e.category_id = $2 AND e.status = 'confirmed'
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $3
	`, userID, categoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses by category: %w", err)
	}
	defer rows.Close()

	return scanExpenses(rows)
}

// GetTotalByUserIDAndCategory calculates total spending for confirmed expenses in a category.
func (r *ExpenseRepository) GetTotalByUserIDAndCategory(
	ctx context.Context,
	userID int64,
	categoryID int,
) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM expenses
		WHERE user_id = $1 AND category_id = $2 AND status = 'confirmed'
	`, userID, categoryID).Scan(&total)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get total by category: %w", err)
	}
	return total, nil
}

// Update modifies an existing expense.
func (r *ExpenseRepository) Update(ctx context.Context, expense *models.Expense) error {
	_, err := r.db.Exec(ctx, `
		UPDATE expenses SET
			amount = $2,
			currency = $3,
			description = $4,
			merchant = $5,
			category_id = $6,
			receipt_file_id = $7,
			status = $8,
			updated_at = NOW()
		WHERE id = $1
	`, expense.ID, expense.Amount, expense.Currency, expense.Description,
		expense.Merchant, expense.CategoryID, expense.ReceiptFileID, expense.Status)
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}
	return nil
}

// Delete removes an expense by ID.
func (r *ExpenseRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM expenses WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete expense: %w", err)
	}
	return nil
}

// DeleteExpiredDrafts removes draft expenses older than the specified duration.
// Returns the number of deleted rows.
func (r *ExpenseRepository) DeleteExpiredDrafts(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.Exec(ctx, `
		DELETE FROM expenses
		WHERE status = $1 AND created_at < $2
	`, models.ExpenseStatusDraft, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired drafts: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// GetTotalByUserIDAndDateRange calculates total spending for confirmed expenses in a date range.
func (r *ExpenseRepository) GetTotalByUserIDAndDateRange(
	ctx context.Context,
	userID int64,
	startDate, endDate time.Time,
) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM expenses
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND status = 'confirmed'
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
			&exp.ID, &exp.UserExpenseNumber, &exp.UserID, &exp.Amount, &exp.Currency, &exp.Description,
			&exp.Merchant, &categoryID, &exp.ReceiptFileID, &exp.Status, &exp.CreatedAt, &exp.UpdatedAt,
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
