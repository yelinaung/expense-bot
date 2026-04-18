package bot

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// TestNormalizeCurrencyCodeIdempotent: norm(norm(x)) == norm(x).
func TestNormalizeCurrencyCodeIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		once := normalizeCurrencyCode(s)
		twice := normalizeCurrencyCode(once)
		if once != twice {
			t.Fatalf("not idempotent: once=%q twice=%q (in=%q)", once, twice, s)
		}
	})
}

// TestNormalizeCurrencyCodeUppercaseTrimmed: output is uppercased and has no
// leading/trailing whitespace.
func TestNormalizeCurrencyCodeUppercaseTrimmed(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		got := normalizeCurrencyCode(s)
		if got != strings.ToUpper(got) {
			t.Fatalf("not uppercased: %q", got)
		}
		if got != strings.TrimSpace(got) {
			t.Fatalf("not trimmed: %q", got)
		}
	})
}

// TestGetCurrencyOrCodeSymbolSupportedReturnsSymbol: for supported codes, returns symbol.
func TestGetCurrencyOrCodeSymbolSupportedReturnsSymbol(t *testing.T) {
	codes := make([]string, 0, len(appmodels.SupportedCurrencies))
	for c := range appmodels.SupportedCurrencies {
		codes = append(codes, c)
	}
	rapid.Check(t, func(t *rapid.T) {
		code := rapid.SampledFrom(codes).Draw(t, "code")
		got := getCurrencyOrCodeSymbol(code)
		want := appmodels.SupportedCurrencies[code]
		if got != want {
			t.Fatalf("getCurrencyOrCodeSymbol(%q) = %q, want %q", code, got, want)
		}
	})
}

// TestGetCurrencyOrCodeSymbolUnknownReturnsCode: unsupported code → returns code verbatim.
func TestGetCurrencyOrCodeSymbolUnknownReturnsCode(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary string, skip if it happens to be a known code.
		code := rapid.String().Draw(t, "code")
		if _, ok := appmodels.SupportedCurrencies[code]; ok {
			t.Skip("known code")
		}
		if appmodels.SupportedCurrencies[code] != "" {
			t.Skip("known code")
		}
		got := getCurrencyOrCodeSymbol(code)
		if got != code {
			t.Fatalf("getCurrencyOrCodeSymbol(%q) = %q, want %q", code, got, code)
		}
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
	rapid.Check(t, func(t *rapid.T) {
		desc := rapid.StringMatching(`[A-Za-z ]{0,20}`).Draw(t, "desc")
		origAmt := genAmount().Draw(t, "origAmt")
		convAmt := genAmount().Draw(t, "convAmt")
		rate := genAmount().Draw(t, "rate")
		origCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "origCur")
		convCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "convCur")
		rateDate := "2026-04-18"

		got := appendOriginalAmountDescription(desc, origAmt, origCur, convAmt, convCur, rate, rateDate)

		if !strings.Contains(got, "orig:") {
			t.Fatalf("missing 'orig:' marker: %q", got)
		}
		if !strings.Contains(got, origCur) {
			t.Fatalf("missing origCur %q: %q", origCur, got)
		}
		if !strings.Contains(got, convCur) {
			t.Fatalf("missing convCur %q: %q", convCur, got)
		}
		if !strings.Contains(got, rateDate) {
			t.Fatalf("missing rateDate: %q", got)
		}

		trimmed := strings.TrimSpace(desc)
		if trimmed != "" {
			if !strings.HasPrefix(got, desc+" ") {
				t.Fatalf("prefix not preserved: desc=%q got=%q", desc, got)
			}
		}
	})
}

// TestAppendConversionUnavailableDescriptionInvariants: contains marker and currencies.
func TestAppendConversionUnavailableDescriptionInvariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		desc := rapid.StringMatching(`[A-Za-z ]{0,20}`).Draw(t, "desc")
		origCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "origCur")
		targetCur := rapid.StringMatching(`[A-Z]{3}`).Draw(t, "targetCur")

		got := appendConversionUnavailableDescription(desc, origCur, targetCur)

		if !strings.Contains(got, "fx_unavailable") {
			t.Fatalf("missing fx_unavailable marker: %q", got)
		}
		if !strings.Contains(got, origCur) {
			t.Fatalf("missing origCur %q: %q", origCur, got)
		}
		if !strings.Contains(got, targetCur) {
			t.Fatalf("missing targetCur %q: %q", targetCur, got)
		}

		trimmed := strings.TrimSpace(desc)
		if trimmed != "" {
			if !strings.HasPrefix(got, desc+" ") {
				t.Fatalf("prefix not preserved: desc=%q got=%q", desc, got)
			}
		}
	})
}
