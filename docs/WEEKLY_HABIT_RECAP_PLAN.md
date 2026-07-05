# Weekly Habit Recap Plan

## Context

The spending reflection feature (`/review` + `/habit`, see
`SPENDING_REFLECTION_PLAN.md`) is live, but the recap is pull-only: users must
remember to run `/habit`. The doc's "Future Extensions" list includes a
scheduled reflection recap. This plan delivers that as a **weekly** push,
riding on the existing weekly report loop: when a user receives their weekly
expense summary, they also receive their spending reflection recap for the
same previous week.

Goal: users see their worth-it / not-worth-it patterns weekly without asking,
using only data they already reviewed. No new data model changes are needed.

## Design

### Delivery

Piggyback on the existing weekly report loop rather than adding a second
ticker loop:

- `startWeeklyReportLoop` / `checkAndSendWeeklyReports` /
  `processWeeklyReportUser` in `internal/bot/weekly_report.go` already handle
  per-user timezone gating (`WeeklyReportDay`/`WeeklyReportHour`), weekly
  dedup (`sent map[int64]string` keyed by previous-week start), pruning, and
  OTel metrics.
- In `processWeeklyReportUser`, after `sendWeeklySummary` returns
  `(true, nil)`, call a new `sendWeeklyHabitRecap(ctx, user, userNow)`.
- The recap is **best-effort**: a recap failure is logged but does not block
  marking the week as sent. This keeps at-most-once semantics for the weekly
  summary (no risk of double summary on retry).
- If the weekly summary is skipped (no expenses that week), the recap is
  skipped too — reviewed expenses in the range are a subset of expenses in
  the range (`GetReviewedByUserIDAndDateRange` filters on `created_at`).

### New function: `sendWeeklyHabitRecap`

Add alongside `sendWeeklySummary` in `internal/bot/weekly_report.go`:

```go
// sendWeeklyHabitRecap sends the previous week's spending reflection
// recap. Returns (false, nil) when there is nothing to send.
func (b *Bot) sendWeeklyHabitRecap(
    ctx context.Context,
    user *appmodels.User,
    userNow time.Time,
) (bool, error)
```

Behavior (all pieces already exist — this is composition only):

1. Range: `getPreviousWeekRangeAt(userNow)` (`internal/bot/date_range.go:84`).
2. Fetch: `b.expenseRepo.GetByUserIDAndDateRange` (total count) and
   `b.expenseRepo.GetReviewedByUserIDAndDateRange` (reflected expenses), same
   as `handleHabitCore` (`internal/bot/handlers_habit.go:100-112`).
3. **Skip silently when there are no reviewed expenses** in the range
   (return `(false, nil)`). A weekly "please run /review" nudge would be
   noise; the recap only appears when the user has reflection data.
4. Analyze: `analyzeExpenseHabit(len(expenses), reviewed, loc, label)`
   (`internal/bot/habit_analyzer.go:62`) with a previous-week label in the
   same style as the weekly summary header, e.g.
   `prevStart.Format("Jan 2") + " to " + prevEnd.AddDate(0,0,-1).Format("Jan 2, 2006")`.
5. Format: `formatHabitSummary(&summary)`
   (`internal/bot/handlers_habit.go:524`) — reuse unchanged so `/habit` and
   the scheduled recap always render identically.
6. Send via `b.messageSender.SendMessage` with `ParseModeHTML`, as
   `sendWeeklySummary` does.

Timezone: use `b.userLocation(user.Timezone)` (already computed in
`processWeeklyReportUser`; pass `loc` in or recompute — match whichever is
cleaner at the call site).

### Config

New env flag, following the `applyWeeklyReportConfig` pattern in
`internal/config/config.go:143`:

- `Config.WeeklyHabitRecapEnabled bool`
- `WEEKLY_HABIT_RECAP_ENABLED=true` to enable; default **false**.
- Effective only when `WEEKLY_REPORT_ENABLED=true`, since the recap rides the
  weekly report loop. Log a startup warning if the recap flag is set while
  the weekly report is disabled.

`processWeeklyReportUser` checks `b.cfg.WeeklyHabitRecapEnabled` before
calling `sendWeeklyHabitRecap`.

### Observability

- Log success/failure with `logger.HashUserID(user.ID)`, mirroring the
  existing weekly report logs.
- Existing `recordWeeklyReportMetrics` continues to cover the loop. Add a
  `BackgroundJobRuns` counter increment with
  `attribute.String("job", "weekly_habit_recap")` on each send attempt
  (status `ok`/`error`) so recap volume is visible separately.

## Files to change

| File | Change |
|------|--------|
| `internal/bot/weekly_report.go` | Add `sendWeeklyHabitRecap`; call it from `processWeeklyReportUser` behind the config flag; recap metrics/logs |
| `internal/config/config.go` | `WeeklyHabitRecapEnabled` field + env parsing in/next to `applyWeeklyReportConfig` |
| `internal/config/config_test.go` | Env parsing tests (enabled/disabled/default), mirroring `TestLoad_WeeklyReport` |
| `internal/bot/weekly_report_test.go` (or new `weekly_habit_recap_test.go`) | Handler-level tests with `mocks.NewMockBot()` |
| `README.md` | Env var table + feature note under spending reflection |
| `.env.example` (if present) | `WEEKLY_HABIT_RECAP_ENABLED` |

No migrations, no repository changes, no new commands, no callback changes.

## Tests

Unit tests using `internal/testutil` mocks (`mocks.NewMockBot()`), table-driven:

- `sendWeeklyHabitRecap`:
  - No expenses in previous week → `(false, nil)`, nothing sent.
  - Expenses but none reviewed → `(false, nil)`, nothing sent.
  - Reviewed expenses → one HTML message sent; body matches
    `formatHabitSummary` output with the previous-week label.
  - Repo error → `(false, err)`.
- `processWeeklyReportUser`:
  - Flag enabled + summary sent → recap message follows the summary.
  - Flag disabled → only the summary is sent.
  - Recap send failure → week still marked in `sent` map, error logged.
- Config: `WEEKLY_HABIT_RECAP_ENABLED` true/false/unset.

Existing `analyzeExpenseHabit` / `formatHabitSummary` tests already cover the
analysis and rendering; do not duplicate them here.

## Verification

```bash
mise run fmt
mise run test
mise run test-race
mise run test-integration   # needs TEST_DATABASE_URL (docker-compose.test.yml)
```

Coverage must stay at or above 50%.

Manual smoke test: run the bot with `WEEKLY_REPORT_ENABLED=true`,
`WEEKLY_HABIT_RECAP_ENABLED=true`, and `WEEKLY_REPORT_DAY`/`WEEKLY_REPORT_HOUR`
set to the current local day/hour; confirm a user with reviewed expenses from
last week receives both the weekly summary and the reflection recap, and a
user with no reviewed expenses receives only the summary.

## Out of scope (future)

- Monthly cadence (would need its own loop or a cadence option).
- Gemini-generated narrative recap.
- Charts comparing worth-it vs not-worth-it spend.
- Per-user opt-in/opt-out command (flag is global for now, consistent with
  the existing weekly report).
