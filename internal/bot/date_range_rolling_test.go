package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetRollingDayRangeAt(t *testing.T) {
	t.Parallel()

	current := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		days      int
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "90 day window ends at current",
			days:      90,
			wantStart: current.AddDate(0, 0, -90),
			wantEnd:   current,
		},
		{
			name:      "non-positive days clamps to one day",
			days:      0,
			wantStart: current.AddDate(0, 0, -1),
			wantEnd:   current,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			start, end := getRollingDayRangeAt(current, tt.days)
			require.Equal(t, tt.wantEnd, end)
			require.Equal(t, tt.wantStart, start)
			require.Equal(t, end.Sub(start), tt.wantEnd.Sub(tt.wantStart))
		})
	}
}
