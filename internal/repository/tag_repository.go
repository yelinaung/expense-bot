package repository

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// TagRepository handles tag database operations.
type TagRepository struct {
	db database.PGXDB
}

// NewTagRepository creates a new TagRepository.
func NewTagRepository(db database.PGXDB) *TagRepository {
	return &TagRepository{db: db}
}

// GetOrCreate inserts a tag if it doesn't exist and returns it.
func (r *TagRepository) GetOrCreate(ctx context.Context, name string) (*models.Tag, error) {
	_, err := r.db.Exec(ctx, `INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to insert tag: %w", err)
	}

	var tag models.Tag
	err = r.db.QueryRow(ctx, `SELECT id, name, created_at FROM tags WHERE name = $1`, name).
		Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}
	return &tag, nil
}

// GetByExpenseID retrieves all tags for an expense.
func (r *TagRepository) GetByExpenseID(ctx context.Context, expenseID int) ([]models.Tag, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name, t.created_at
		FROM tags t
		JOIN expense_tags et ON t.id = et.tag_id
		WHERE et.expense_id = $1
		ORDER BY t.name
	`, expenseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags by expense: %w", err)
	}
	defer rows.Close()

	return scanTags(rows)
}

// GetByExpenseIDs batch-loads tags for multiple expenses.
func (r *TagRepository) GetByExpenseIDs(ctx context.Context, expenseIDs []int) (map[int][]models.Tag, error) {
	if len(expenseIDs) == 0 {
		return make(map[int][]models.Tag), nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT et.expense_id, t.id, t.name, t.created_at
		FROM tags t
		JOIN expense_tags et ON t.id = et.tag_id
		WHERE et.expense_id = ANY($1)
		ORDER BY t.name
	`, expenseIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags by expense IDs: %w", err)
	}
	defer rows.Close()

	result := make(map[int][]models.Tag)
	for rows.Next() {
		var expenseID int
		var tag models.Tag
		if err := rows.Scan(&expenseID, &tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		result[expenseID] = append(result[expenseID], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}
	return result, nil
}

// SetExpenseTags replaces all tags on an expense with the given tag IDs.
func (r *TagRepository) SetExpenseTags(ctx context.Context, expenseID int, tagIDs []int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM expense_tags WHERE expense_id = $1`, expenseID)
	if err != nil {
		return fmt.Errorf("failed to clear expense tags: %w", err)
	}

	for _, tagID := range tagIDs {
		_, err := r.db.Exec(ctx, `
			INSERT INTO expense_tags (expense_id, tag_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, expenseID, tagID)
		if err != nil {
			return fmt.Errorf("failed to add tag %d to expense %d: %w", tagID, expenseID, err)
		}
	}
	return nil
}

// AddTagsToExpense adds tags to an expense without removing existing ones.
func (r *TagRepository) AddTagsToExpense(ctx context.Context, expenseID int, tagIDs []int) error {
	for _, tagID := range tagIDs {
		_, err := r.db.Exec(ctx, `
			INSERT INTO expense_tags (expense_id, tag_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, expenseID, tagID)
		if err != nil {
			return fmt.Errorf("failed to add tag %d to expense %d: %w", tagID, expenseID, err)
		}
	}
	return nil
}

// RemoveTagFromExpense removes a tag from an expense.
func (r *TagRepository) RemoveTagFromExpense(ctx context.Context, expenseID, tagID int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM expense_tags WHERE expense_id = $1 AND tag_id = $2`, expenseID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from expense: %w", err)
	}
	return nil
}

// GetAll retrieves all tags, limited to 100.
func (r *TagRepository) GetAll(ctx context.Context) ([]models.Tag, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, created_at FROM tags ORDER BY name LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	return scanTags(rows)
}

// GetByName retrieves a tag by name (exact match).
func (r *TagRepository) GetByName(ctx context.Context, name string) (*models.Tag, error) {
	var tag models.Tag
	err := r.db.QueryRow(ctx, `SELECT id, name, created_at FROM tags WHERE name = $1`, name).
		Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag by name: %w", err)
	}
	return &tag, nil
}

// Delete removes a tag by ID. CASCADE handles junction rows.
func (r *TagRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// GetExpensesByTagID retrieves confirmed expenses that have a specific tag.
func (r *TagRepository) GetExpensesByTagID(ctx context.Context, userID int64, tagID int, limit int) ([]models.Expense, error) {
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.user_expense_number, e.user_id, e.amount, e.currency, e.description, e.merchant, e.category_id,
		       e.receipt_file_id, e.status, e.created_at, e.updated_at,
		       c.id, c.name, c.created_at
		FROM expenses e
		LEFT JOIN categories c ON e.category_id = c.id
		JOIN expense_tags et ON e.id = et.expense_id
		WHERE et.tag_id = $1 AND e.user_id = $2 AND e.status = 'confirmed'
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $3
	`, tagID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses by tag: %w", err)
	}
	defer rows.Close()

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

// scanTags is a helper to scan tag rows.
func scanTags(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]models.Tag, error) {
	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}
	return tags, nil
}
