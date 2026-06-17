package bot

import (
	"cmp"
	"slices"
	"time"

	"github.com/shopspring/decimal"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

type spendingDriver string

const (
	minCategoryReflectionSample = 2
	spendingDriverOther         = spendingDriver("Other")
)

// Keep this list append-only and in stable order because callbacks encode indices.
var spendingDrivers = []spendingDriver{
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

type habitSummary struct {
	PeriodLabel                 string
	TotalCount                  int
	ReviewedCount               int
	WorthItCount                int
	NotWorthItCount             int
	WorthItByCurrency           map[string]decimal.Decimal
	NotWorthItByCurrency        map[string]decimal.Decimal
	TopDriver                   spendingDriver
	BestValueCategory           string
	MostRegrettedCategory       string
	BusiestNotWorthItWeekday    time.Weekday
	HasBusiestNotWorthItWeekday bool
}

type categoryReflectionStats struct {
	reviewed int
	worth    int
	notWorth int
}

type categoryRank struct {
	name        string
	rate        float64
	targetCount int
}

func analyzeExpenseHabit(
	totalCount int,
	reviewedExpenses []appmodels.Expense,
	loc *time.Location,
	periodLabel string,
) habitSummary {
	loc = normalizeLocation(loc)
	summary := habitSummary{
		PeriodLabel:          periodLabel,
		TotalCount:           totalCount,
		WorthItByCurrency:    make(map[string]decimal.Decimal),
		NotWorthItByCurrency: make(map[string]decimal.Decimal),
	}

	driverCounts := make(map[spendingDriver]int)
	categoryStats := make(map[string]categoryReflectionStats)
	weekdayCounts := make(map[time.Weekday]int)

	for i := range reviewedExpenses {
		expense := &reviewedExpenses[i]
		if expense.WorthIt == nil {
			continue
		}

		summary.ReviewedCount++
		categoryName := habitCategoryName(expense)
		stats := categoryStats[categoryName]
		stats.reviewed++

		if expense.SpendDriver != nil && *expense.SpendDriver != "" {
			driverCounts[spendingDriver(*expense.SpendDriver)]++
		}

		if *expense.WorthIt {
			summary.WorthItCount++
			stats.worth++
			summary.WorthItByCurrency[expense.Currency] = summary.WorthItByCurrency[expense.Currency].Add(expense.Amount)
		} else {
			summary.NotWorthItCount++
			stats.notWorth++
			summary.NotWorthItByCurrency[expense.Currency] = summary.NotWorthItByCurrency[expense.Currency].Add(expense.Amount)
			weekdayCounts[expense.CreatedAt.In(loc).Weekday()]++
		}

		categoryStats[categoryName] = stats
	}

	summary.TopDriver = topSpendingDriver(driverCounts)
	summary.BestValueCategory = bestValueCategory(categoryStats)
	summary.MostRegrettedCategory = mostRegrettedCategory(categoryStats)
	summary.BusiestNotWorthItWeekday, summary.HasBusiestNotWorthItWeekday = busiestWeekday(weekdayCounts)

	return summary
}

func habitCategoryName(expense *appmodels.Expense) string {
	if expense.Category != nil && expense.Category.Name != "" {
		return expense.Category.Name
	}
	return categoryUncategorized
}

func topSpendingDriver(counts map[spendingDriver]int) spendingDriver {
	var top spendingDriver
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
	ranks := make([]categoryRank, 0, len(stats))
	for categoryName, stat := range stats {
		if stat.reviewed >= minCategoryReflectionSample {
			targetCount := numerator(stat)
			ranks = append(ranks, categoryRank{
				name:        categoryName,
				rate:        float64(targetCount) / float64(stat.reviewed),
				targetCount: targetCount,
			})
		}
	}

	slices.SortFunc(ranks, func(a, b categoryRank) int {
		if rateOrder := cmp.Compare(b.rate, a.rate); rateOrder != 0 {
			return rateOrder
		}
		if countOrder := cmp.Compare(b.targetCount, a.targetCount); countOrder != 0 {
			return countOrder
		}
		return cmp.Compare(a.name, b.name)
	})

	if len(ranks) == 0 {
		return ""
	}
	return ranks[0].name
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
