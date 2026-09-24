package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

var fixedDisplayLocation = time.UTC

var fixedCreatedAt = time.Date(2026, 9, 9, 8, 45, 0, 0, time.UTC)

func newDisplayTestBot() *Bot {
	return &Bot{displayLocation: fixedDisplayLocation}
}

// TestFormatExpenseListItemPrefersEditedDescriptionAfterEdit reproduces the
// regression introduced by d0078eb: after a same-currency /edit, Description
// is updated ("latte") while Merchant keeps its pre-edit value ("coffee").
// The list renderer must surface the edited description rather than the
// stale merchant.
func TestFormatExpenseListItemPrefersEditedDescriptionAfterEdit(t *testing.T) {
	t.Parallel()
	b := newDisplayTestBot()
	exp := &appmodels.Expense{
		UserExpenseNumber: 1,
		Amount:            decimal.RequireFromString("42.00"),
		Currency:          "SGD",
		Description:       "latte",
		Merchant:          "coffee",
		CreatedAt:         fixedCreatedAt,
	}
	got := b.formatExpenseListItem(exp, nil)
	require.Contains(t, got, "- latte", "list should reflect the edited description")
	require.NotContains(t, got, "coffee", "stale merchant must not appear in the list")
	require.Contains(t, got, "#1")
	require.Contains(t, got, "S$42.00 SGD")
	require.Contains(t, got, "<i>Sep 9 08:45</i>")
}

// TestFormatExpenseListItemCrossCurrencyShowsMerchant is the cross-currency
// control: a freshly created cross-currency row stores the FX note in
// Description (e.g. "latte [orig: ...]") and the bare merchant name in
// Merchant. The renderer must keep the FX note hidden and show the clean
// name, even though it now reads from Description.
func TestFormatExpenseListItemCrossCurrencyShowsMerchant(t *testing.T) {
	t.Parallel()
	b := newDisplayTestBot()
	exp := &appmodels.Expense{
		UserExpenseNumber: 1,
		Amount:            decimal.RequireFromString("7.40"),
		Currency:          "SGD",
		Description:       "latte [orig: USD 5.50 -> SGD 7.40 @ 1.3600 (2026-02-14)]",
		Merchant:          "latte",
		CreatedAt:         fixedCreatedAt,
	}
	got := b.formatExpenseListItem(exp, nil)
	require.Contains(t, got, "- latte", "cross-currency rows show the clean name")
	require.NotContains(t, got, "[orig:", "cross-currency FX note must stay hidden")
	require.NotContains(t, got, "USD 5.50", "FX note contents must not leak into the list")
}

// TestFormatExpenseListItemCrossCurrencyAfterEdit verifies a cross-currency
// row after /edit: Description is the raw new name (no FX note re-added by
// applyParsedEdit) while Merchant keeps the original name. The list must
// show the new name, not the stale merchant.
func TestFormatExpenseListItemCrossCurrencyAfterEdit(t *testing.T) {
	t.Parallel()
	b := newDisplayTestBot()
	exp := &appmodels.Expense{
		UserExpenseNumber: 2,
		Amount:            decimal.RequireFromString("7.40"),
		Currency:          "SGD",
		Description:       "newname",
		Merchant:          "oldname",
		CreatedAt:         fixedCreatedAt,
	}
	got := b.formatExpenseListItem(exp, nil)
	require.Contains(t, got, "- newname", "edited cross-currency row shows the new description")
	require.NotContains(t, got, "oldname", "stale merchant must not show after a cross-currency edit")
}

// TestFormatExpenseListItemFallsBackToMerchantWhenDescriptionEmpty locks in
// the fallback path for rows that legitimately have no Description (e.g.
// receipt-driven expenses that only populated Merchant). The Merchant must
// still be displayed when Description is empty.
func TestFormatExpenseListItemFallsBackToMerchantWhenDescriptionEmpty(t *testing.T) {
	t.Parallel()
	b := newDisplayTestBot()
	exp := &appmodels.Expense{
		UserExpenseNumber: 1,
		Amount:            decimal.RequireFromString("3.20"),
		Currency:          "SGD",
		Description:       "",
		Merchant:          "Receipt Merchant",
		CreatedAt:         fixedCreatedAt,
	}
	got := b.formatExpenseListItem(exp, nil)
	require.Contains(t, got, "- Receipt Merchant", "merchant fallback must be used when description is empty")
}

// TestBuildExpenseListMessageReflectsEditedDescription verifies the shared
// message builder used by /list, /today, /week, /category, daily reminders
// and weekly reports surfaces the edited description end-to-end while
// keeping FX notes hidden.
func TestBuildExpenseListMessageReflectsEditedDescription(t *testing.T) {
	t.Parallel()
	b := newDisplayTestBot()
	expenses := []appmodels.Expense{
		{
			UserExpenseNumber: 1,
			Amount:            decimal.RequireFromString("42.00"),
			Currency:          "SGD",
			Description:       "latte",
			Merchant:          "coffee",
			CreatedAt:         fixedCreatedAt,
		},
		{
			UserExpenseNumber: 2,
			Amount:            decimal.RequireFromString("7.40"),
			Currency:          "SGD",
			Description:       "latte [orig: USD 5.50 -> SGD 7.40 @ 1.3600 (2026-02-14)]",
			Merchant:          "latte",
			CreatedAt:         fixedCreatedAt,
		},
	}
	got := b.buildExpenseListMessage("Today's Expenses", expenses, nil)
	require.Contains(t, got, "Today's Expenses")
	require.Contains(t, got, "#1 S$42.00 SGD - latte")
	require.Contains(t, got, "#2 S$7.40 SGD - latte")
	require.NotContains(t, got, "coffee", "the full list must reflect edited descriptions")
	require.NotContains(t, got, "[orig:", "the full list must keep FX notes hidden")
	require.Equal(t, 2, strings.Count(got, " - latte\n"))
}
