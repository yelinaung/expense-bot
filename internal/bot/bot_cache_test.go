package bot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/models"
)

func TestGetCategoriesWithCache(t *testing.T) {
	// NOTE: No t.Parallel() - database tests must run sequentially

	t.Run("cache miss - fetches from DB", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)

		// Ensure cache is empty
		b.categoryCache = nil
		b.categoryCacheExpiry = time.Time{}

		// First call should fetch from DB
		categories, err := b.getCategoriesWithCache(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		// Cache should now be populated
		require.NotNil(t, b.categoryCache)
		require.True(t, time.Now().Before(b.categoryCacheExpiry))
	})

	t.Run("cache hit - uses cached data", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)

		// First call to populate cache
		categories1, err := b.getCategoriesWithCache(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, categories1)

		// Store cache timestamp
		firstCacheTime := b.categoryCacheExpiry

		// Second call should use cache (no DB query)
		categories2, err := b.getCategoriesWithCache(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, categories2)

		// Cache timestamp should not have changed
		require.Equal(t, firstCacheTime, b.categoryCacheExpiry)

		// Should return same data
		require.Len(t, categories2, len(categories1))
	})

	t.Run("cache expiry - refetches from DB", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)

		// Populate cache with expired timestamp
		b.categoryCache = []models.Category{{ID: 1, Name: "Test"}}
		b.categoryCacheExpiry = time.Now().Add(-1 * time.Minute) // Expired

		// Call should refetch from DB
		categories, err := b.getCategoriesWithCache(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		// Cache should be updated with new expiry
		require.True(t, time.Now().Before(b.categoryCacheExpiry))
	})

	t.Run("concurrent access - no race conditions", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)

		// Clear cache
		b.categoryCache = nil
		b.categoryCacheExpiry = time.Time{}

		// Multiple goroutines accessing cache simultaneously
		done := make(chan bool, 10)
		for range 10 {
			go func() {
				_, err := b.getCategoriesWithCache(context.Background())
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all goroutines
		for range 10 {
			<-done
		}

		// Cache should be populated
		require.NotNil(t, b.categoryCache)
	})
}

func TestInvalidateCategoryCache(t *testing.T) {
	// NOTE: No t.Parallel() - database tests must run sequentially

	t.Run("clears cache", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)

		// Populate cache
		b.categoryCache = []models.Category{{ID: 1, Name: "Test"}}
		b.categoryCacheExpiry = time.Now().Add(5 * time.Minute)

		// Invalidate
		b.invalidateCategoryCache()

		// Cache should be cleared
		require.Nil(t, b.categoryCache)
		require.True(t, b.categoryCacheExpiry.IsZero())
	})

	t.Run("next access refetches from DB", func(t *testing.T) {
		pool := TestDB(t)
		b := setupTestBot(t, pool)

		// Populate cache
		categories1, err := b.getCategoriesWithCache(context.Background())
		require.NoError(t, err)

		// Invalidate
		b.invalidateCategoryCache()

		// Next access should refetch
		categories2, err := b.getCategoriesWithCache(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, categories2)

		// Should have fresh data
		require.Len(t, categories2, len(categories1))
	})
}
