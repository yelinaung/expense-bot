package bot

import "time"

// normalizeLocation returns loc, or time.Local when loc is nil to preserve
// runtime-local behavior. Callers that need deterministic behavior should pass
// an explicit location.
func normalizeLocation(loc *time.Location) *time.Location {
	if loc == nil {
		return time.Local
	}

	return loc
}

// getDayDateRangeAt returns today's range in loc as a [start, end) interval.
func getDayDateRangeAt(loc *time.Location, now time.Time) (time.Time, time.Time) {
	safeLoc := normalizeLocation(loc)
	current := now.In(safeLoc)
	startOfDay := time.Date(
		current.Year(),
		current.Month(),
		current.Day(),
		0,
		0,
		0,
		0,
		safeLoc,
	)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	return startOfDay, endOfDay
}

// getWeekDateRangeAt returns the current week range in loc as [start, end).
// Week starts on Monday.
func getWeekDateRangeAt(loc *time.Location, now time.Time) (time.Time, time.Time) {
	safeLoc := normalizeLocation(loc)
	current := now.In(safeLoc)
	weekday := int(current.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	startOfWeek := time.Date(
		current.Year(),
		current.Month(),
		current.Day()-weekday+1,
		0,
		0,
		0,
		0,
		safeLoc,
	)
	endOfWeek := startOfWeek.AddDate(0, 0, 7)

	return startOfWeek, endOfWeek
}

// getMonthDateRangeAt returns the current month range in loc as [start, end).
func getMonthDateRangeAt(loc *time.Location, now time.Time) (time.Time, time.Time) {
	safeLoc := normalizeLocation(loc)
	current := now.In(safeLoc)
	startOfMonth := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, safeLoc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	return startOfMonth, endOfMonth
}
