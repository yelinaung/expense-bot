package bot

import "time"

// normalizeLocation returns loc, or runtime local timezone when loc is nil.
func normalizeLocation(loc *time.Location) *time.Location {
	if loc == nil {
		return time.Local //nolint:gosmopolitan // Preserve existing runtime-local fallback semantics.
	}

	return loc
}

// getDayDateRangeAt returns today's range as a [start, end) interval.
// current must already be in the desired display location.
func getDayDateRangeAt(current time.Time) (time.Time, time.Time) {
	loc := current.Location()
	startOfDay := time.Date(
		current.Year(),
		current.Month(),
		current.Day(),
		0,
		0,
		0,
		0,
		loc,
	)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	return startOfDay, endOfDay
}

// getWeekDateRangeAt returns the current week range as [start, end).
// current must already be in the desired display location.
// Week starts on Monday.
func getWeekDateRangeAt(current time.Time) (time.Time, time.Time) {
	loc := current.Location()
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
		loc,
	)
	endOfWeek := startOfWeek.AddDate(0, 0, 7)

	return startOfWeek, endOfWeek
}

// getMonthDateRangeAt returns the current month range as [start, end).
// current must already be in the desired display location.
func getMonthDateRangeAt(current time.Time) (time.Time, time.Time) {
	loc := current.Location()
	startOfMonth := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	return startOfMonth, endOfMonth
}
