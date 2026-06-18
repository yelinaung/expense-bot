package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"hegel.dev/go/hegel"
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

// hegelLocationGen is the Hegel analog of genLocation: a fixed offset location
// between -12h and +14h.
func hegelLocationGen() hegel.Generator[*time.Location] {
	return hegel.Composite(func(tc hegel.TestCase) *time.Location {
		hours := hegel.Draw(tc, hegel.Integers(-12, 14))
		return time.FixedZone("gen", hours*3600)
	})
}

// hegelTimeInLocationGen is the Hegel analog of genTimeInLocation: a time within
// 2000..2040 attached to a drawn location.
func hegelTimeInLocationGen() hegel.Generator[time.Time] {
	return hegel.Composite(func(tc hegel.TestCase) time.Time {
		loc := hegel.Draw(tc, hegelLocationGen())
		year := hegel.Draw(tc, hegel.Integers(2000, 2040))
		month := hegel.Draw(tc, hegel.Integers(1, 12))
		day := hegel.Draw(tc, hegel.Integers(1, 28))
		hour := hegel.Draw(tc, hegel.Integers(0, 23))
		minute := hegel.Draw(tc, hegel.Integers(0, 59))
		second := hegel.Draw(tc, hegel.Integers(0, 59))
		return time.Date(year, time.Month(month), day, hour, minute, second, 0, loc)
	})
}

// TestHegelNormalizeLocationNonNilPassthrough is the Hegel equivalent: non-nil
// input is returned verbatim.
func TestHegelNormalizeLocationNonNilPassthrough(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		loc := hegel.Draw(ht, hegelLocationGen())
		require.Same(ht, loc, normalizeLocation(loc))
	})
}

// TestHegelGetDayDateRangeAtInvariants is the Hegel equivalent of the day-range
// contract.
func TestHegelGetDayDateRangeAtInvariants(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cur := hegel.Draw(ht, hegelTimeInLocationGen())
		start, end := getDayDateRangeAt(cur)

		require.Equal(ht, 0, start.Hour())
		require.Equal(ht, 0, start.Minute())
		require.Equal(ht, 0, start.Second())
		require.Equal(ht, 0, start.Nanosecond())
		require.Equal(ht, cur.Year(), start.Year())
		require.Equal(ht, cur.Month(), start.Month())
		require.Equal(ht, cur.Day(), start.Day())
		require.Equal(ht, start.AddDate(0, 0, 1), end)
		require.False(ht, cur.Before(start), "cur before start")
		require.True(ht, cur.Before(end), "cur not before end")
		require.Equal(ht, cur.Location(), start.Location())
		require.Equal(ht, cur.Location(), end.Location())
	})
}

// TestHegelGetWeekDateRangeAtInvariants is the Hegel equivalent of the
// week-range contract.
func TestHegelGetWeekDateRangeAtInvariants(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cur := hegel.Draw(ht, hegelTimeInLocationGen())
		start, end := getWeekDateRangeAt(cur)

		require.Equal(ht, time.Monday, start.Weekday(), "start=%s", start)
		require.Equal(ht, start.AddDate(0, 0, 7), end)
		require.Equal(ht, 0, start.Hour())
		require.Equal(ht, 0, start.Minute())
		require.Equal(ht, 0, start.Second())
		require.Equal(ht, 0, start.Nanosecond())
		require.False(ht, cur.Before(start), "cur=%s start=%s", cur, start)
		require.True(ht, cur.Before(end), "cur=%s end=%s", cur, end)
		require.Equal(ht, cur.Location(), start.Location())
	})
}

// TestHegelGetMonthDateRangeAtInvariants is the Hegel equivalent of the
// month-range contract.
func TestHegelGetMonthDateRangeAtInvariants(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		cur := hegel.Draw(ht, hegelTimeInLocationGen())
		start, end := getMonthDateRangeAt(cur)

		require.Equal(ht, 1, start.Day())
		require.Equal(ht, 0, start.Hour())
		require.Equal(ht, 0, start.Minute())
		require.Equal(ht, 0, start.Second())
		require.Equal(ht, 0, start.Nanosecond())
		require.Equal(ht, cur.Year(), start.Year())
		require.Equal(ht, cur.Month(), start.Month())
		require.Equal(ht, start.AddDate(0, 1, 0), end)
		require.False(ht, cur.Before(start))
		require.True(ht, cur.Before(end))
		require.Equal(ht, cur.Location(), start.Location())
	})
}
