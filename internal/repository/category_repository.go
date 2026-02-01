// Package repository provides database access for domain entities.
package repository

import (
	"context"
	"fmt"

	"gitlab.com/yelinaung/expense-bot/internal/database"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// CategoryRepository handles category database operations.
type CategoryRepository struct {
	db database.PGXDB
}

// NewCategoryRepository creates a new CategoryRepository.
func NewCategoryRepository(db database.PGXDB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAll retrieves all categories.
func (r *CategoryRepository) GetAll(ctx context.Context) ([]models.Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, created_at FROM categories ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var cat models.Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, cat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}
	return categories, nil
}

// GetByID retrieves a category by ID.
func (r *CategoryRepository) GetByID(ctx context.Context, id int) (*models.Category, error) {
	var cat models.Category
	err := r.db.QueryRow(ctx, `
		SELECT id, name, created_at FROM categories WHERE id = $1
	`, id).Scan(&cat.ID, &cat.Name, &cat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return &cat, nil
}

// GetByName retrieves a category by name (case-insensitive).
func (r *CategoryRepository) GetByName(ctx context.Context, name string) (*models.Category, error) {
	var cat models.Category
	err := r.db.QueryRow(ctx, `
		SELECT id, name, created_at FROM categories WHERE LOWER(name) = LOWER($1)
	`, name).Scan(&cat.ID, &cat.Name, &cat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get category by name: %w", err)
	}
	return &cat, nil
}

// Create adds a new category.
func (r *CategoryRepository) Create(ctx context.Context, name string) (*models.Category, error) {
	var cat models.Category
	err := r.db.QueryRow(ctx, `
		INSERT INTO categories (name) VALUES ($1)
		RETURNING id, name, created_at
	`, name).Scan(&cat.ID, &cat.Name, &cat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return &cat, nil
}

// Update modifies an existing category name.
func (r *CategoryRepository) Update(ctx context.Context, id int, name string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE categories SET name = $2 WHERE id = $1
	`, id, name)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}
	return nil
}

// Delete removes a category by ID.
func (r *CategoryRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}
