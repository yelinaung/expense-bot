package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/config"
)

func TestNewExchangeService(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		ExchangeRateBaseURL:  "https://api.frankfurter.app",
		ExchangeRateTimeout:  5 * time.Second,
		ExchangeRateCacheTTL: time.Hour,
	}

	svc := newExchangeService(cfg)
	require.NotNil(t, svc)
}
