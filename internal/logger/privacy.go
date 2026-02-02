package logger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

var hashSalt string

func init() {
	// Load salt from environment or generate a default one
	// In production, set LOG_HASH_SALT environment variable
	hashSalt = os.Getenv("LOG_HASH_SALT")
	if hashSalt == "" {
		hashSalt = "default-salt-change-in-production"
	}
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
