# Spending Reflection Feature Plan

## Goal

Build a Peek-inspired spending reflection feature for the Telegram bot.
Instead of only categorizing expenses by where money went, the bot should help
users understand whether each expense felt worth it and what drove the purchase.

Peek's public product framing centers on a simple per-transaction question:
"worth it, or not", spending drivers such as convenience, ritual, comfort, and
impulse, then narrative recaps and realistic plans based on actual spending plus
the user's own evaluation.

Reference: https://peek.money/

## Product Scope

### MVP Commands

Add two user-facing commands:

- `/review` - Review recent unreviewed expenses.
- `/habit [week|month|90d]` - Show a spending reflection recap.

Default `/habit` period should be `month`.

### Review Flow

When a user runs `/review`, show one unreviewed confirmed expense at a time with
inline buttons:

- Worth it
- Not worth it
- Skip

After the user chooses worth it or not worth it, ask what drove the purchase:

- Necessity
- Convenience
- Ritual
- Comfort
- Celebration
- Social
- Gift
- Self-care
- Hobby
- Impulse
- Productivity
- Other

The bot should save the reflection and move to the next unreviewed expense.

### Inline Reflection After Save

After a new expense is saved, include a small reflection entry point in every
expense confirmation message: structured `/add`, free-text entry, receipt OCR,
voice input, and inline edited confirmation where appropriate. This should not
interrupt the fast expense entry flow. Confirm that the final keyboard remains
readable beside existing edit and delete actions; a third row is acceptable if
it keeps the main actions clear.

Example buttons:

- Worth it
- Not worth it
- Later

If the user chooses worth it or not worth it, ask for the spending driver using
the same driver buttons as `/review`. If the user taps Later, leave the expense
unreviewed and edit the message keyboard to remove the reflection buttons while
keeping the normal confirmation text intact.

### Habit Recap

`/habit month` should generate a concise, narrative text recap. For the MVP,
do not convert currencies in the read path. Show totals per currency and keep
category calculations separated by currency where spend totals matter.

```text
📈 Spending Reflection - Last 30 days

Reviewed: 23/41 expenses
Worth-it rate: 74%
Worth-it spend:
  SGD: S$612.40
  USD: $42.00
Not-worth-it spend:
  SGD: S$119.80

Top driver: Convenience
Best-value category: Food - Grocery
Most regretted category: Shopping
Weekday pattern: Friday had the most not-worth-it spending

Insight: Small convenience purchases added up most often when you were busy.
```

The recap should be descriptive, not judgmental. Avoid telling the user what
they should do. Use language like "pattern", "often", and "looks like" instead
of "bad", "waste", or "problem".

## Data Model

Append nullable reflection columns to the migration statement slice in
`internal/database/migrations.go`:

```sql
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS worth_it BOOLEAN;
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS spend_driver TEXT;
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;
```

Update `models.Expense` with:

```go
WorthIt     *bool
SpendDriver *string
ReviewedAt  *time.Time
```

Keep the fields nullable so old expenses are valid and users can skip review.
Do not model `spend_driver` as a plain string unless every query uses
`COALESCE` to an empty string; pgx cannot scan SQL NULL into a Go string.

## Repository Changes

Extend `ExpenseRepository` with:

```go
UpdateReflection(
    ctx context.Context,
    expenseID int,
    userID int64,
    worthIt *bool,
    driver string,
) error

GetUnreviewedByUserID(
    ctx context.Context,
    userID int64,
    limit int,
) ([]models.Expense, error)

GetNextUnreviewedByUserID(
    ctx context.Context,
    userID int64,
    afterExpenseID int,
) (*models.Expense, error)

GetReviewedByUserIDAndDateRange(
    ctx context.Context,
    userID int64,
    startDate time.Time,
    endDate time.Time,
) ([]models.Expense, error)
```

Repository behavior:

- Only operate on `status = 'confirmed'`.
- Scope `UpdateReflection` by both `id` and `user_id`. Existing generic
  `Update` and `Delete` methods currently scope by `id`; tightening those is
  worth a separate hardening change but is outside this feature MVP.
- Treat skipped expenses as still unreviewed for future sessions, but do not
  re-serve the same skipped expense in the current callback flow.
- Join categories in the same way existing expense list queries do.
- `UpdateReflection` should set `reviewed_at = NOW()` in SQL rather than
  accepting a timestamp from Go.
- `UpdateReflection` should write SQL NULL for an empty driver string, not an
  empty text value, so it round-trips consistently with `SpendDriver *string`.
- Reflection queries must not reuse `scanExpenses` unchanged. Either update
  `scanExpenses` and add `worth_it`, `spend_driver`, and `reviewed_at` to
  every existing expense SELECT list, or create a separate
  `scanExpensesWithReflection` helper for the new queries. The separate helper
  is lower risk for the MVP.
- Scan nullable reflection columns into pointer fields or nullable locals.

`GetNextUnreviewedByUserID` should prevent the skip flow from looping on the
same row. Use the current expense as a cursor and return the next unreviewed
expense after it in the same stable ordering used by `GetUnreviewedByUserID`
(`created_at DESC, id DESC`). If the skipped expense is the last unreviewed
expense, the handler should show `No more expenses to review.`

## Bot Changes

### Command Registration

Update bot command registration in `internal/bot/bot.go`:

- Add `/review` with description `Review recent expenses`.
- Add `/habit` with description `Show spending reflection recap`.

Register handlers:

- `/review` -> `handleReview`
- `/habit` -> `handleHabit`

### Help Text

Update `/help` in `internal/bot/handlers_commands.go` with a new section:

```text
Spending Reflection:
• /review - Mark recent expenses as worth it or not worth it
• /habit [week|month|90d] - Show your spending reflection recap
```

### New Handler File

Create `internal/bot/handlers_habit.go` for:

- `handleReview`
- `handleReviewCore`
- `handleHabit`
- `handleHabitCore`
- callback handling for worth-it and driver buttons
- message formatting helpers

Callback prefixes should be explicit and stable:

- `review_worth_`
- `review_not_worth_`
- `review_skip_`
- `review_driver_`

Prefer registering one `review_` callback prefix in `internal/bot/bot.go` using
`HandlerTypeCallbackQueryData` and `MatchTypePrefix`, then dispatch internally
with `strings.HasPrefix`. Registering each concrete prefix separately also
works, but one prefix keeps registration boilerplate smaller.

The callback payload should include all state needed to continue the flow. Do
not use the in-memory `pendingEdits` map for this feature. It is keyed by chat
ID, lost on restart, and unnecessary here.

Recommended callback data:

```text
review_worth_<expenseID>
review_not_worth_<expenseID>
review_skip_<expenseID>
review_driver_<expenseID>_<0|1>_<driverIdx>
```

Use `0` for not worth it and `1` for worth it. Use a stable numeric driver
index rather than the driver label, because labels can contain hyphens or other
delimiters. The driver list order becomes an append-only contract: add new
drivers to the end and do not reorder existing entries, otherwise old callback
messages may decode to a different driver. These payloads are well under the
Telegram callback data limit of 64 bytes for normal integer IDs.

When parsing callback data, prefer `strings.TrimPrefix` for each registered
prefix and split only the remaining payload. Avoid relying on fixed indexes
from `strings.Split(data, "_")` over the full callback string because the
prefixes themselves contain underscores.

### Analyzer File

Create `internal/bot/habit_analyzer.go` for pure business logic.

Suggested types:

```go
type SpendingDriver string

type HabitSummary struct {
    PeriodLabel string
    ReviewedCount int
    TotalCount int
    WorthItCount int
    NotWorthItCount int
    WorthItByCurrency map[string]decimal.Decimal
    NotWorthItByCurrency map[string]decimal.Decimal
    WorthItByCategory map[string]map[string]decimal.Decimal
    NotWorthItByCategory map[string]map[string]decimal.Decimal
    TopDriver SpendingDriver
    BestValueCategory string
    MostRegrettedCategory string
    BusiestNotWorthItWeekday time.Weekday
    Insight string
}
```

Use `github.com/shopspring/decimal` for all money totals.

Keep the analyzer deterministic:

- Sort ties alphabetically.
- For weekdays, sort by count, then weekday order.
- For empty data, return a clear empty summary instead of an error.
- Apply a minimum reviewed-sample guard before naming "best-value" or
  "most-regretted" categories. A category with only one reviewed expense should
  not win these labels unless there is no stronger candidate.

Category scoring rules:

- Best-value category: among categories with at least two reviewed expenses,
  choose the highest worth-it rate. Tie-break by higher worth-it count, then
  alphabetical category name.
- Most-regretted category: among categories with at least two reviewed expenses,
  choose the highest not-worth-it rate. Tie-break by higher not-worth-it count,
  then alphabetical category name.
- If no category reaches the minimum sample size, omit the category label or use
  `Not enough reviewed category data yet.`

For MVP category totals, keep maps keyed by currency first and category second,
for example `map["SGD"]["Food - Grocery"] = decimal.NewFromFloat(120.50)`.
This avoids invalid cross-currency sums. A future version may convert all spend
to the user default currency before analysis.

## Period Semantics

Use existing timezone-aware helpers where possible and add a helper for the
rolling window:

```go
func getRollingDayRangeAt(current time.Time, days int) (time.Time, time.Time)
```

`90d` should call this helper with `days = 90`.

- `week` uses the current week from Monday to Sunday.
- `month` uses the current calendar month.
- `90d` uses a rolling 90-day window ending at `now`.

For previous-period comparison:

- `week`: previous calendar week.
- `month`: previous calendar month.
- `90d`: previous rolling 90 days.

Because this is a personal reflection feature, `/habit` should use
`b.userLocation(user.Timezone)`, as daily reminders and weekly reports do. This
reuses the existing empty-or-invalid-timezone fallback to `b.displayLocation`
instead of reimplementing that logic.

## Insight Rules

The MVP does not need Gemini. Generate deterministic insights from reviewed
data:

- If no reviewed expenses: ask the user to run `/review`.
- If worth-it rate is high: mention the strongest positive pattern.
- If not-worth-it spend is concentrated in one category: mention that category.
- If not-worth-it spend is concentrated on one weekday: mention the weekday.
- If no strong pattern exists: report the top driver and worth-it rate.
- If multiple currencies are present, mention that spend totals are grouped by
  currency.

Example:

```text
Insight: Your not-worth-it spending was mostly Shopping on Fridays.
```

## Tests

### Unit Tests

Add tests for `habit_analyzer.go`:

- Empty expense list.
- All unreviewed expenses.
- Mixed worth-it and not-worth-it expenses.
- Multiple currencies produce separate currency totals and do not cross-sum.
- Category totals are grouped by currency first, then category.
- Nil category uses the existing `categoryUncategorized` constant rather than
  hardcoding a separate string.
- Tie-breaking is deterministic.
- Weekday pattern uses expense `CreatedAt` in the selected user timezone.

### Repository Tests

Add tests in `internal/repository/expense_repository_test.go` or a dedicated
file:

- `UpdateReflection` updates only the owner's expense.
- `GetUnreviewedByUserID` returns only confirmed unreviewed expenses.
- `GetReviewedByUserIDAndDateRange` filters by user, date range, status, and
  non-null `reviewed_at`.
- Draft expenses are excluded.
- Nullable `spend_driver`, `worth_it`, and `reviewed_at` scan without errors.
- Reflection queries use `scanExpensesWithReflection` or an updated shared
  scanner with matching SELECT columns.

Database tests should not use `t.Parallel()`.

Add migration tests that assert the three new columns exist. The existing
migration tests check column existence rather than strict column counts, so new
columns should not break them.

### Handler Tests

Add bot handler tests:

- `/review` with no expenses.
- `/review` with unreviewed expenses sends reflection buttons.
- Worth-it callback asks for driver using callback-encoded state.
- Driver callback saves reflection using the encoded expense ID, worth flag, and
  driver index.
- Skip callback advances to the next unreviewed expense instead of re-serving
  the skipped one.
- `/habit` default period works.
- `/habit invalid` returns usage.
- `/habit month` with no reviewed expenses asks the user to review expenses.
- `/habit` arg parsing uses `extractCommandArgs`.

## Documentation Updates

Update `README.md`:

- Feature list.
- Basic commands table.
- Usage section for `/review` and `/habit`.
- Database schema section with reflection fields.

## Verification

Run the required project checks:

```bash
mise run fmt
mise run test
mise run test-coverage
mise run test-race
mise run test-integration
```

Coverage must stay at or above 50%.

## Future Extensions

After the MVP is stable:

- Scheduled monthly reflection recap.
- User-configurable driver list.
- Optional Gemini-generated narrative recap from deterministic summary data.
- Monthly "realistic caps" based on worth-it spend, not total spend.
- Charts comparing worth-it and not-worth-it spend by category.
- A skipped-review state if users want to permanently ignore certain expenses.
