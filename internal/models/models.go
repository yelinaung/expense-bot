// Package models defines the domain entities for the expense tracker.
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// User represents a Telegram user.
type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Category represents an expense category.
type Category struct {
	ID        int
	Name      string
	CreatedAt time.Time
}

// Expense represents a single expense entry.
type Expense struct {
	ID            int
	UserID        int64
	Amount        decimal.Decimal
	Currency      string
	Description   string
	CategoryID    *int
	Category      *Category
	ReceiptFileID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
