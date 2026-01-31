//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/shopspring/decimal"
	"gitlab.com/yelinaung/expense-bot/internal/bot"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func main() {
	expenses := []models.Expense{
		{Amount: decimal.NewFromFloat(150.50), Category: &models.Category{Name: "Food - Groceries"}},
		{Amount: decimal.NewFromFloat(130.50), Category: &models.Category{Name: "Food - Dining Out"}},
		{Amount: decimal.NewFromFloat(60.00), Category: &models.Category{Name: "Transportation"}},
		{Amount: decimal.NewFromFloat(25.00), Category: &models.Category{Name: "Entertainment"}},
		{Amount: decimal.NewFromFloat(120.00), Category: &models.Category{Name: "Utilities"}},
	}

	chartData, err := bot.GenerateExpenseChart(expenses, "January 2026")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("graph.png", chartData, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Created graph.png - Example expense breakdown chart")
}
