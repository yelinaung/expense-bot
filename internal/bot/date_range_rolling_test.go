package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetRollingDayRangeAt(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	t.Run("90 day window ends at current", func(t *testing.T) {
		t.Parallel()
		start, end := getRollingDayRangeAt(current, 90)
		require.Equal(t, current, end)
		require.Equal(t, current.AddDate(0, 0, -90), start)
		require.Equal(t, 90*24*time.Hour, end.Sub(start))
	})

	t.Run("non-positive days clamps to one day", func(t *testing.T) {
		t.Parallel()
		start, end := getRollingDayRangeAt(current, 0)
		require.Equal(t, current, end)
		require.Equal(t, current.AddDate(0, 0, -1), start)
	})
}
