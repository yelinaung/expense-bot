package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatGreeting(t *testing.T) {
	t.Parallel()

	t.Run("returns empty for empty name", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "", formatGreeting(""))
	})

	t.Run("returns formatted greeting with name", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, ", John", formatGreeting("John"))
	})

	t.Run("handles name with spaces", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, ", John Doe", formatGreeting("John Doe"))
	})
}
