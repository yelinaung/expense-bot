package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// genLocation draws a fixed offset location between -12h and +14h.
func genLocation() *rapid.Generator[*time.Location] {
	return rapid.Custom(func(t *rapid.T) *time.Location {
		hours := rapid.IntRange(-12, 14).Draw(t, "hours")
		return time.FixedZone("gen", hours*3600)
	})
}

// genTimeInLocation generates a time within 2000..2040 attached to a drawn location.
func genTimeInLocation() *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		loc := genLocation().Draw(t, "loc")
		year := rapid.IntRange(2000, 2040).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 28).Draw(t, "day")
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		minute := rapid.IntRange(0, 59).Draw(t, "minute")
		second := rapid.IntRange(0, 59).Draw(t, "second")
		return time.Date(year, time.Month(month), day, hour, minute, second, 0, loc)
	})
}

// TestNormalizeLocationNilReturnsLocal: nil → time.Local.
func TestNormalizeLocationNilReturnsLocal(t *testing.T) {
	t.Parallel()
	require.Equal(t, time.Local, normalizeLocation(nil)) //nolint:gosmopolitan // mirrors prod fallback
}

// TestNormalizeLocationNonNilPassthrough: non-nil input returned verbatim.
func TestNormalizeLocationNonNilPassthrough(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		loc := genLocation().Draw(t, "loc")
		require.Same(t, loc, normalizeLocation(loc))
	})
}

// TestGetDayDateRangeAtInvariants:
//   - start is 00:00:00 on current's calendar date
//   - end == start.AddDate(0,0,1)
//   - start <= current < end
//   - both in same location
func TestGetDayDateRangeAtInvariants(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cur := genTimeInLocation().Draw(t, "cur")
		start, end := getDayDateRangeAt(cur)

		require.Equal(t, 0, start.Hour())
		require.Equal(t, 0, start.Minute())
		require.Equal(t, 0, start.Second())
		require.Equal(t, 0, start.Nanosecond())
		require.Equal(t, cur.Year(), start.Year())
		require.Equal(t, cur.Month(), start.Month())
		require.Equal(t, cur.Day(), start.Day())
		require.Equal(t, start.AddDate(0, 0, 1), end)
		require.False(t, cur.Before(start), "cur before start")
		require.True(t, cur.Before(end), "cur not before end")
		require.Equal(t, cur.Location(), start.Location())
		require.Equal(t, cur.Location(), end.Location())
	})
}

// TestGetWeekDateRangeAtInvariants:
//   - start.Weekday() == Monday
//   - end == start.AddDate(0,0,7)
//   - start <= current < end
//   - start at midnight
func TestGetWeekDateRangeAtInvariants(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cur := genTimeInLocation().Draw(t, "cur")
		start, end := getWeekDateRangeAt(cur)

		require.Equal(t, time.Monday, start.Weekday(), "start=%s", start)
		require.Equal(t, start.AddDate(0, 0, 7), end)
		require.Equal(t, 0, start.Hour())
		require.Equal(t, 0, start.Minute())
		require.Equal(t, 0, start.Second())
		require.Equal(t, 0, start.Nanosecond())
		require.False(t, cur.Before(start), "cur=%s start=%s", cur, start)
		require.True(t, cur.Before(end), "cur=%s end=%s", cur, end)
		require.Equal(t, cur.Location(), start.Location())
	})
}

// TestGetMonthDateRangeAtInvariants:
//   - start.Day() == 1
//   - start at midnight
//   - end == start.AddDate(0,1,0)
//   - start <= current < end
func TestGetMonthDateRangeAtInvariants(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		cur := genTimeInLocation().Draw(t, "cur")
		start, end := getMonthDateRangeAt(cur)

		require.Equal(t, 1, start.Day())
		require.Equal(t, 0, start.Hour())
		require.Equal(t, 0, start.Minute())
		require.Equal(t, 0, start.Second())
		require.Equal(t, 0, start.Nanosecond())
		require.Equal(t, cur.Year(), start.Year())
		require.Equal(t, cur.Month(), start.Month())
		require.Equal(t, start.AddDate(0, 1, 0), end)
		require.False(t, cur.Before(start))
		require.True(t, cur.Before(end))
		require.Equal(t, cur.Location(), start.Location())
	})
}
