package logger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

var hashSalt string

// MinHashSaltLength is the minimum required length for LOG_HASH_SALT.
const MinHashSaltLength = 32

// InitHashSalt initializes the hash salt from environment.
// This should be called during application startup after config validation.
// Panics if LOG_HASH_SALT is missing or too short.
func InitHashSalt() {
	hashSalt = os.Getenv("LOG_HASH_SALT")
	if hashSalt == "" {
		panic("LOG_HASH_SALT environment variable is required (generate with: openssl rand -hex 32)")
	}
	if len(hashSalt) < MinHashSaltLength {
		panic(fmt.Sprintf("LOG_HASH_SALT must be at least %d characters for adequate entropy", MinHashSaltLength))
	}
}

// InitHashSaltForTesting sets a test salt for unit tests only.
// This should never be used in production code.
func InitHashSaltForTesting(salt string) {
	hashSalt = salt
}

// HashUserID creates a privacy-preserving hash of a user ID.
// This allows tracking user actions without exposing actual user IDs.
func HashUserID(userID int64) string {
	data := fmt.Sprintf("%d:%s", userID, hashSalt)
	hash := sha256.Sum256([]byte(data))
	// Return first 8 characters for readability
	return hex.EncodeToString(hash[:])[:8]
}

// HashChatID creates a privacy-preserving hash of a chat ID.
func HashChatID(chatID int64) string {
	data := fmt.Sprintf("%d:%s", chatID, hashSalt)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:8]
}

// SanitizeDescription removes or truncates sensitive information from descriptions.
// This redacts the description but preserves length information for debugging.
func SanitizeDescription(desc string) string {
	if desc == "" {
		return "<empty>"
	}

	// Preserve length info but redact content
	words := strings.Fields(desc)
	wordCount := len(words)
	charCount := len(desc)

	return fmt.Sprintf("<redacted: %d words, %d chars>", wordCount, charCount)
}

// SanitizeText is a general-purpose sanitizer for any user-provided text.
func SanitizeText(text string) string {
	if text == "" {
		return "<empty>"
	}

	// For short text, show first few characters
	if len(text) <= 10 {
		return fmt.Sprintf("<%d chars>", len(text))
	}

	// For longer text, show prefix and length
	return fmt.Sprintf("%s...<%d chars>", text[:3], len(text))
}
