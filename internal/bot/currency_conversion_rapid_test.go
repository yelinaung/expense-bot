package bot

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// TestNormalizeCurrencyCodeIdempotent: norm(norm(x)) == norm(x).
func TestNormalizeCurrencyCodeIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		once := normalizeCurrencyCode(s)
		twice := normalizeCurrencyCode(once)
		require.Equal(t, once, twice, "not idempotent (in=%q)", s)
	})
}

// TestNormalizeCurrencyCodeUppercaseTrimmed: output is uppercased and trimmed.
func TestNormalizeCurrencyCodeUppercaseTrimmed(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		got := normalizeCurrencyCode(s)
		require.Equal(t, strings.ToUpper(got), got, "not uppercased: %q", got)
		require.Equal(t, strings.TrimSpace(got), got, "not trimmed: %q", got)
	})
}

// TestGetCurrencyOrCodeSymbolSupportedReturnsSymbol: for supported codes, returns symbol.
func TestGetCurrencyOrCodeSymbolSupportedReturnsSymbol(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		code := genSupportedCurrency().Draw(t, "code")
		got := getCurrencyOrCodeSymbol(code)
		want := appmodels.SupportedCurrencies[code]
		require.Equal(t, want, got, "code=%q", code)
	})
}

// TestGetCurrencyOrCodeSymbolUnknownReturnsCode: unsupported code → returns code verbatim.
func TestGetCurrencyOrCodeSymbolUnknownReturnsCode(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		code := rapid.String().Draw(t, "code")
		if _, ok := appmodels.SupportedCurrencies[code]; ok {
			t.Skip("known code")
		}
		if appmodels.SupportedCurrencies[code] != "" {
			t.Skip("known code")
		}
		got := getCurrencyOrCodeSymbol(code)
		require.Equal(t, code, got, "code=%q", code)
	})
}

// genAmount generates a positive decimal with up to 4 fractional digits.
func genAmount() *rapid.Generator[decimal.Decimal] {
	return rapid.Custom(func(t *rapid.T) decimal.Decimal {
		v := rapid.IntRange(1, 1_000_000).Draw(t, "v")
		exp := rapid.IntRange(-4, 2).Draw(t, "exp")
		return decimal.New(int64(v), int32(exp))
	})
}

// TestAppendOriginalAmountDescriptionInvariants:
//   - Result contains "orig:" marker
//   - Result contains original currency and converted currency codes
//   - Non-empty trimmed description is preserved as prefix
func TestAppendOriginalAmountDescriptionInvariants(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		desc := rapid.StringMatching(`[A-Za-z ]{0,20}`).Draw(t, "desc")
		origAmt := genAmount().Draw(t, "origAmt")
		convAmt := genAmount().Draw(t, "convAmt")
		rate := genAmount().Draw(t, "rate")
		origCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "origCur")
		convCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "convCur")
		rateDate := "2026-04-18"

		got := appendOriginalAmountDescription(desc, origAmt, origCur, convAmt, convCur, rate, rateDate)

		require.Contains(t, got, "orig:", "missing marker")
		require.Contains(t, got, origCur)
		require.Contains(t, got, convCur)
		require.Contains(t, got, rateDate)

		if strings.TrimSpace(desc) != "" {
			require.True(t, strings.HasPrefix(got, desc+" "),
				"prefix not preserved: desc=%q got=%q", desc, got)
		}
	})
}

// TestAppendConversionUnavailableDescriptionInvariants: contains marker and currencies.
func TestAppendConversionUnavailableDescriptionInvariants(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		desc := rapid.StringMatching(`[A-Za-z ]{0,20}`).Draw(t, "desc")
		origCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "origCur")
		targetCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "targetCur")

		got := appendConversionUnavailableDescription(desc, origCur, targetCur)

		require.Contains(t, got, "fx_unavailable")
		require.Contains(t, got, origCur)
		require.Contains(t, got, targetCur)

		if strings.TrimSpace(desc) != "" {
			require.True(t, strings.HasPrefix(got, desc+" "),
				"prefix not preserved: desc=%q got=%q", desc, got)
		}
	})
}
