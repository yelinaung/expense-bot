package bot

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"hegel.dev/go/hegel"
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

// hegelAmountGen generates a positive decimal with up to 4 fractional digits,
// the Hegel analog of genAmount. Shared shape: v in [1, 1_000_000], exp in
// [-4, 2]. The bounds prevent whole*100+frac overflow in test arithmetic
// (see Generator Discipline: "draw a smaller type so test arithmetic can't
// overflow"); the decimal domain itself is the contract's full valid range.
func hegelAmountGen() hegel.Generator[decimal.Decimal] {
	return hegel.Composite(func(tc hegel.TestCase) decimal.Decimal {
		v := hegel.Draw(tc, hegel.Integers(1, 1_000_000))
		exp := hegel.Draw(tc, hegel.Integers(-4, 2))
		return decimal.New(int64(v), int32(exp))
	})
}

// TestHegelNormalizeCurrencyCodeIdempotent is the Hegel equivalent of the
// currency-code normalizer idempotence contract over full-Unicode input.
func TestHegelNormalizeCurrencyCodeIdempotent(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.Text())
		once := normalizeCurrencyCode(s)
		twice := normalizeCurrencyCode(once)
		require.Equal(ht, once, twice, "not idempotent (in=%q)", s)
	})
}

// TestHegelNormalizeCurrencyCodeUppercaseTrimmed is the Hegel equivalent:
// output is uppercased and trimmed, over full-Unicode input.
func TestHegelNormalizeCurrencyCodeUppercaseTrimmed(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		s := hegel.Draw(ht, hegel.Text())
		got := normalizeCurrencyCode(s)
		require.Equal(ht, strings.ToUpper(got), got, "not uppercased: %q", got)
		require.Equal(ht, strings.TrimSpace(got), got, "not trimmed: %q", got)
	})
}

// TestHegelGetCurrencyOrCodeSymbolSupportedReturnsSymbol is the Hegel equivalent:
// for supported codes, returns the symbol.
func TestHegelGetCurrencyOrCodeSymbolSupportedReturnsSymbol(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		code := hegel.Draw(ht, hegel.SampledFrom(sortedSupportedCurrencyCodes()))
		got := getCurrencyOrCodeSymbol(code)
		want := appmodels.SupportedCurrencies[code]
		require.Equal(ht, want, got, "code=%q", code)
	})
}

// TestHegelGetCurrencyOrCodeSymbolUnknownReturnsCode is the Hegel equivalent:
// unsupported code returns the code verbatim. Hegel draws full-Unicode strings;
// codes that happen to collide with a supported currency are rejected via
// Assume so the property only runs over the "unknown" domain.
func TestHegelGetCurrencyOrCodeSymbolUnknownReturnsCode(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		code := hegel.Draw(ht, hegel.Text())
		_, known := appmodels.SupportedCurrencies[code]
		ht.Assume(!known)
		ht.Assume(appmodels.SupportedCurrencies[code] == "")

		got := getCurrencyOrCodeSymbol(code)
		require.Equal(ht, code, got, "code=%q", code)
	})
}

// TestHegelAppendOriginalAmountDescriptionInvariants is the Hegel equivalent of
// the conversion-metadata description invariants.
func TestHegelAppendOriginalAmountDescriptionInvariants(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		desc := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{0,20}`, true))
		origAmt := hegel.Draw(ht, hegelAmountGen())
		convAmt := hegel.Draw(ht, hegelAmountGen())
		rate := hegel.Draw(ht, hegelAmountGen())
		origCur := hegel.Draw(ht, hegel.FromRegex(`[A-Z]{3}`, true))
		convCur := hegel.Draw(ht, hegel.FromRegex(`[A-Z]{3}`, true))
		rateDate := "2026-04-18"

		got := appendOriginalAmountDescription(desc, origAmt, origCur, convAmt, convCur, rate, rateDate)

		require.Contains(ht, got, "orig:", "missing marker")
		require.Contains(ht, got, origCur)
		require.Contains(ht, got, convCur)
		require.Contains(ht, got, rateDate)

		if strings.TrimSpace(desc) != "" {
			require.True(ht, strings.HasPrefix(got, desc+" "),
				"prefix not preserved: desc=%q got=%q", desc, got)
		}
	})
}

// TestHegelAppendConversionUnavailableDescriptionInvariants is the Hegel
// equivalent: contains marker and currencies.
func TestHegelAppendConversionUnavailableDescriptionInvariants(t *testing.T) {
	t.Parallel()
	hegel.Test(t, func(ht *hegel.T) {
		desc := hegel.Draw(ht, hegel.FromRegex(`[A-Za-z ]{0,20}`, true))
		origCur := hegel.Draw(ht, hegel.FromRegex(`[A-Z]{3}`, true))
		targetCur := hegel.Draw(ht, hegel.FromRegex(`[A-Z]{3}`, true))

		got := appendConversionUnavailableDescription(desc, origCur, targetCur)

		require.Contains(ht, got, "fx_unavailable")
		require.Contains(ht, got, origCur)
		require.Contains(ht, got, targetCur)

		if strings.TrimSpace(desc) != "" {
			require.True(ht, strings.HasPrefix(got, desc+" "),
				"prefix not preserved: desc=%q got=%q", desc, got)
		}
	})
}
