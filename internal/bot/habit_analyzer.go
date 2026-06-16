package bot

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

type SpendingDriver string

const (
	minCategoryReflectionSample = 2
	spendingDriverOther         = SpendingDriver("Other")
)

// Keep this list append-only and in stable order because callbacks encode indices.
var spendingDrivers = []SpendingDriver{
	"Necessity",
	"Convenience",
	"Ritual",
	"Comfort",
	"Celebration",
	"Social",
	"Gift",
	"Self-care",
	"Hobby",
	"Impulse",
	"Productivity",
	spendingDriverOther,
}

type HabitSummary struct {
	PeriodLabel                     string
	TotalCount                      int
	ReviewedCount                   int
	WorthItCount                    int
	NotWorthItCount                 int
	WorthItByCurrency               map[string]decimal.Decimal
	NotWorthItByCurrency            map[string]decimal.Decimal
	WorthItByCategoryAndCurrency    map[string]map[string]decimal.Decimal
	NotWorthItByCategoryAndCurrency map[string]map[string]decimal.Decimal
	TopDriver                       SpendingDriver
	BestValueCategory               string
	MostRegrettedCategory           string
	BusiestNotWorthItWeekday        time.Weekday
	HasBusiestNotWorthItWeekday     bool
	Insight                         string
}

type categoryReflectionStats struct {
	reviewed int
	worth    int
	notWorth int
}

func AnalyzeExpenseHabit(
	allExpenses []appmodels.Expense,
	reviewedExpenses []appmodels.Expense,
	loc *time.Location,
	periodLabel string,
) HabitSummary {
	loc = normalizeLocation(loc)
	summary := HabitSummary{
		PeriodLabel:                     periodLabel,
		TotalCount:                      len(allExpenses),
		WorthItByCurrency:               make(map[string]decimal.Decimal),
		NotWorthItByCurrency:            make(map[string]decimal.Decimal),
		WorthItByCategoryAndCurrency:    make(map[string]map[string]decimal.Decimal),
		NotWorthItByCategoryAndCurrency: make(map[string]map[string]decimal.Decimal),
	}

	driverCounts := make(map[SpendingDriver]int)
	categoryStats := make(map[string]categoryReflectionStats)
	weekdayCounts := make(map[time.Weekday]int)

	for i := range reviewedExpenses {
		expense := reviewedExpenses[i]
		if expense.WorthIt == nil {
			continue
		}

		summary.ReviewedCount++
		categoryName := habitCategoryName(&expense)
		stats := categoryStats[categoryName]
		stats.reviewed++

		if expense.SpendDriver != nil && *expense.SpendDriver != "" {
			driverCounts[SpendingDriver(*expense.SpendDriver)]++
		}

		if *expense.WorthIt {
			summary.WorthItCount++
			stats.worth++
			summary.WorthItByCurrency[expense.Currency] = summary.WorthItByCurrency[expense.Currency].Add(expense.Amount)
			addCategoryCurrencyTotal(summary.WorthItByCategoryAndCurrency, categoryName, expense.Currency, expense.Amount)
		} else {
			summary.NotWorthItCount++
			stats.notWorth++
			summary.NotWorthItByCurrency[expense.Currency] = summary.NotWorthItByCurrency[expense.Currency].Add(expense.Amount)
			addCategoryCurrencyTotal(summary.NotWorthItByCategoryAndCurrency, categoryName, expense.Currency, expense.Amount)
			weekdayCounts[expense.CreatedAt.In(loc).Weekday()]++
		}

		categoryStats[categoryName] = stats
	}

	summary.TopDriver = topSpendingDriver(driverCounts)
	summary.BestValueCategory = bestValueCategory(categoryStats)
	summary.MostRegrettedCategory = mostRegrettedCategory(categoryStats)
	summary.BusiestNotWorthItWeekday, summary.HasBusiestNotWorthItWeekday = busiestWeekday(weekdayCounts)
	summary.Insight = buildHabitInsight(&summary)

	return summary
}

func habitCategoryName(expense *appmodels.Expense) string {
	if expense.Category != nil && expense.Category.Name != "" {
		return expense.Category.Name
	}
	return categoryUncategorized
}

func addCategoryCurrencyTotal(
	totals map[string]map[string]decimal.Decimal,
	categoryName string,
	currency string,
	amount decimal.Decimal,
) {
	if totals[categoryName] == nil {
		totals[categoryName] = make(map[string]decimal.Decimal)
	}
	totals[categoryName][currency] = totals[categoryName][currency].Add(amount)
}

func topSpendingDriver(counts map[SpendingDriver]int) SpendingDriver {
	var top SpendingDriver
	bestCount := 0
	for driver, count := range counts {
		if count > bestCount || (count == bestCount && (top == "" || string(driver) < string(top))) {
			top = driver
			bestCount = count
		}
	}
	return top
}

func bestValueCategory(stats map[string]categoryReflectionStats) string {
	return rankedCategory(stats, func(s categoryReflectionStats) int { return s.worth })
}

func mostRegrettedCategory(stats map[string]categoryReflectionStats) string {
	return rankedCategory(stats, func(s categoryReflectionStats) int { return s.notWorth })
}

func rankedCategory(stats map[string]categoryReflectionStats, numerator func(categoryReflectionStats) int) string {
	categories := make([]string, 0, len(stats))
	for categoryName, stat := range stats {
		if stat.reviewed >= minCategoryReflectionSample {
			categories = append(categories, categoryName)
		}
	}
	sort.Strings(categories)

	bestCategory := ""
	bestNumerator := -1
	bestDenominator := 1
	bestCount := -1
	for _, categoryName := range categories {
		stat := stats[categoryName]
		currentNumerator := numerator(stat)
		if bestCategory == "" || currentNumerator*bestDenominator > bestNumerator*stat.reviewed ||
			(currentNumerator*bestDenominator == bestNumerator*stat.reviewed && currentNumerator > bestCount) {
			bestCategory = categoryName
			bestNumerator = currentNumerator
			bestDenominator = stat.reviewed
			bestCount = currentNumerator
		}
	}
	return bestCategory
}

func busiestWeekday(counts map[time.Weekday]int) (time.Weekday, bool) {
	var top time.Weekday
	bestCount := 0
	for weekday, count := range counts {
		if count > bestCount || (count == bestCount && weekday < top) {
			top = weekday
			bestCount = count
		}
	}
	return top, bestCount > 0
}

func buildHabitInsight(summary *HabitSummary) string {
	switch {
	case summary.ReviewedCount == 0:
		return "Review a few expenses with /review to build your spending reflection."
	case summary.MostRegrettedCategory != "":
		return "Your not-worth-it spending showed up most often in " + summary.MostRegrettedCategory + "."
	case summary.TopDriver != "":
		return "Your most common spending driver was " + string(summary.TopDriver) + "."
	default:
		return "Keep reviewing expenses to reveal stronger spending patterns."
	}
}
