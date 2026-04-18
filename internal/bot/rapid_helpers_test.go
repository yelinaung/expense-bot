package bot

import (
	"sort"
	"sync"

	"gitlab.com/yelinaung/expense-bot/internal/models"
	"pgregory.net/rapid"
)

// sortedSupportedCurrencyCodes returns models.SupportedCurrencies keys sorted
// once. Caching keeps rapid.Check loops from rebuilding the slice per iteration
// while preserving deterministic order across runs.
var sortedSupportedCurrencyCodes = sync.OnceValue(func() []string {
	codes := make([]string, 0, len(models.SupportedCurrencies))
	for c := range models.SupportedCurrencies {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
})

// genSupportedCurrency draws a currency code from models.SupportedCurrencies.
func genSupportedCurrency() *rapid.Generator[string] {
	return rapid.SampledFrom(sortedSupportedCurrencyCodes())
}
