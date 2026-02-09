// Package models defines the domain entities for the expense tracker.
package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// DefaultCurrency is the default currency for new users.
const DefaultCurrency = "SGD"

// MaxCategoryNameLength is the maximum allowed length for category names.
const MaxCategoryNameLength = 50

// SupportedCurrencies lists all supported currency codes.
var SupportedCurrencies = map[string]string{
	"SGD": "S$",
	"USD": "$",
	"EUR": "€",
	"GBP": "£",
	"JPY": "¥",
	"CNY": "¥",
	"MYR": "RM",
	"THB": "฿",
	"IDR": "Rp",
	"PHP": "₱",
	"VND": "₫",
	"KRW": "₩",
	"INR": "₹",
	"AUD": "A$",
	"NZD": "NZ$",
	"HKD": "HK$",
	"TWD": "NT$",
}

// User represents a Telegram user.
type User struct {
	ID              int64
	Username        string
	FirstName       string
	LastName        string
	DefaultCurrency string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Category represents an expense category.
type Category struct {
	ID        int
	Name      string
	CreatedAt time.Time
}

// ExpenseStatus represents the status of an expense.
const (
	ExpenseStatusDraft     = "draft"
	ExpenseStatusConfirmed = "confirmed"
)

// MaxTagNameLength is the maximum allowed length for tag names.
const MaxTagNameLength = 30

// Tag represents an expense tag/label.
type Tag struct {
	ID        int
	Name      string
	CreatedAt time.Time
}

// ApprovedUser represents a dynamically approved bot user.
type ApprovedUser struct {
	ID         int
	UserID     int64
	Username   string
	ApprovedBy int64
	CreatedAt  time.Time
}

// Expense represents a single expense entry.
type Expense struct {
	ID                int
	UserExpenseNumber int64
	UserID            int64
	Amount            decimal.Decimal
	Currency          string
	Description       string
	Merchant          string
	CategoryID        *int
	Category          *Category
	Tags              []Tag
	ReceiptFileID     string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
