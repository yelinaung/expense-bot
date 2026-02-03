package logger

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Initialize hash salt for all tests in this package.
	InitHashSaltForTesting("test-salt-for-unit-tests-minimum-32-chars")
	os.Exit(m.Run())
}

func TestHashUserID(t *testing.T) {
	t.Run("produces consistent hash for same user ID", func(t *testing.T) {
		hash1 := HashUserID(12345)
		hash2 := HashUserID(12345)
		require.Equal(t, hash1, hash2)
	})

	t.Run("produces different hashes for different user IDs", func(t *testing.T) {
		hash1 := HashUserID(12345)
		hash2 := HashUserID(67890)
		require.NotEqual(t, hash1, hash2)
	})

	t.Run("produces 8 character hash", func(t *testing.T) {
		hash := HashUserID(12345)
		require.Len(t, hash, 8)
	})

	t.Run("changes hash when salt changes", func(t *testing.T) {
		originalSalt := hashSalt
		defer func() { hashSalt = originalSalt }()

		hash1 := HashUserID(12345)

		hashSalt = "different-salt"
		hash2 := HashUserID(12345)

		require.NotEqual(t, hash1, hash2)
	})
}

func TestHashChatID(t *testing.T) {
	t.Run("produces consistent hash for same chat ID", func(t *testing.T) {
		hash1 := HashChatID(12345)
		hash2 := HashChatID(12345)
		require.Equal(t, hash1, hash2)
	})

	t.Run("produces different hashes for different chat IDs", func(t *testing.T) {
		hash1 := HashChatID(12345)
		hash2 := HashChatID(67890)
		require.NotEqual(t, hash1, hash2)
	})
}

func TestSanitizeDescription(t *testing.T) {
	t.Run("redacts empty description", func(t *testing.T) {
		result := SanitizeDescription("")
		require.Equal(t, "<empty>", result)
	})

	t.Run("shows word and character count", func(t *testing.T) {
		result := SanitizeDescription("lunch at hawker center")
		require.Contains(t, result, "4 words")
		require.Contains(t, result, "22 chars")
	})

	t.Run("handles single word", func(t *testing.T) {
		result := SanitizeDescription("coffee")
		require.Contains(t, result, "1 words")
		require.Contains(t, result, "6 chars")
	})

	t.Run("preserves length information for debugging", func(t *testing.T) {
		desc := "expensive dinner with clients"
		result := SanitizeDescription(desc)
		require.Contains(t, result, "4 words")
		require.Contains(t, result, "29 chars")
		require.NotContains(t, result, "dinner")
		require.NotContains(t, result, "clients")
	})
}

func TestSanitizeText(t *testing.T) {
	t.Run("redacts empty text", func(t *testing.T) {
		result := SanitizeText("")
		require.Equal(t, "<empty>", result)
	})

	t.Run("shows length for short text", func(t *testing.T) {
		result := SanitizeText("short")
		require.Equal(t, "<5 chars>", result)
	})

	t.Run("shows prefix for longer text", func(t *testing.T) {
		result := SanitizeText("this is a long text")
		require.Contains(t, result, "thi...")
		require.Contains(t, result, "19 chars")
	})
}

func TestInitHashSalt(t *testing.T) {
	t.Run("panics when LOG_HASH_SALT is missing", func(t *testing.T) {
		originalSalt := hashSalt
		defer func() { hashSalt = originalSalt }()

		t.Setenv("LOG_HASH_SALT", "")

		require.Panics(t, func() {
			InitHashSalt()
		})
	})

	t.Run("panics when LOG_HASH_SALT is too short", func(t *testing.T) {
		originalSalt := hashSalt
		defer func() { hashSalt = originalSalt }()

		t.Setenv("LOG_HASH_SALT", "short")

		require.Panics(t, func() {
			InitHashSalt()
		})
	})

	t.Run("succeeds with valid LOG_HASH_SALT", func(t *testing.T) {
		originalSalt := hashSalt
		defer func() { hashSalt = originalSalt }()

		validSalt := "this-is-a-valid-salt-with-at-least-32-characters"
		t.Setenv("LOG_HASH_SALT", validSalt)

		require.NotPanics(t, func() {
			InitHashSalt()
		})
		require.Equal(t, validSalt, hashSalt)
	})
}

func TestInitHashSaltForTesting(t *testing.T) {
	t.Run("sets hash salt directly", func(t *testing.T) {
		originalSalt := hashSalt
		defer func() { hashSalt = originalSalt }()

		testSalt := "test-salt"
		InitHashSaltForTesting(testSalt)

		require.Equal(t, testSalt, hashSalt)
	})
}
