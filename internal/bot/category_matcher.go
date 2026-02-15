package bot

import (
	"strings"

	"gitlab.com/yelinaung/expense-bot/internal/models"
)

// MatchCategory finds the best matching category for a suggested category name.
// Matching strategy:
// 1. Exact match (case-insensitive)
// 2. Contains match (e.g., "dining" matches "Food - Dining Out")
// 3. No match -> returns nil.
func MatchCategory(suggested string, categories []models.Category) *models.Category {
	suggestedLower := strings.ToLower(strings.TrimSpace(suggested))
	if suggestedLower == "" {
		return nil
	}

	if match := findExactCategoryMatch(suggested, categories); match != nil {
		return match
	}
	if match := findShortestContainingCategoryMatch(suggestedLower, categories); match != nil {
		return match
	}
	if match := findLongestContainedCategoryMatch(suggestedLower, categories); match != nil {
		return match
	}
	return findWordBasedCategoryMatch(suggested, categories)
}

func findExactCategoryMatch(suggested string, categories []models.Category) *models.Category {
	for i := range categories {
		if strings.EqualFold(categories[i].Name, suggested) {
			return &categories[i]
		}
	}
	return nil
}

func findShortestContainingCategoryMatch(suggestedLower string, categories []models.Category) *models.Category {
	var bestMatch *models.Category
	bestLen := 0
	for i := range categories {
		catLower := strings.ToLower(categories[i].Name)
		if strings.Contains(catLower, suggestedLower) {
			if bestMatch == nil || len(categories[i].Name) < bestLen {
				bestMatch = &categories[i]
				bestLen = len(categories[i].Name)
			}
		}
	}
	return bestMatch
}

func findLongestContainedCategoryMatch(suggestedLower string, categories []models.Category) *models.Category {
	var bestMatch *models.Category
	bestLen := 0
	for i := range categories {
		catLower := strings.ToLower(categories[i].Name)
		if strings.Contains(suggestedLower, catLower) {
			if bestMatch == nil || len(categories[i].Name) > bestLen {
				bestMatch = &categories[i]
				bestLen = len(categories[i].Name)
			}
		}
	}
	return bestMatch
}

func findWordBasedCategoryMatch(suggested string, categories []models.Category) *models.Category {
	suggestedWords := extractSignificantWords(suggested)
	for i := range categories {
		catWords := extractSignificantWords(categories[i].Name)
		for _, sw := range suggestedWords {
			for _, cw := range catWords {
				if strings.EqualFold(sw, cw) {
					return &categories[i]
				}
			}
		}
	}
	return nil
}

// extractSignificantWords extracts words from a string, filtering out common separators.
func extractSignificantWords(s string) []string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "/", " ")
	s = strings.ReplaceAll(s, "&", " ")

	words := strings.Fields(s)
	var significant []string

	for _, w := range words {
		if len(w) >= 3 && !isStopWord(w) {
			significant = append(significant, w)
		}
	}

	return significant
}

// isStopWord returns true for common words that shouldn't be matched.
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"and": true,
		"the": true,
		"for": true,
	}
	return stopWords[word]
}
