# Privacy-Preserving Logging

This document explains how to log user actions while preserving privacy.

## Principles

1. **Hash User Identifiers**: Never log raw user IDs or chat IDs
2. **Sanitize User Content**: Redact or truncate user-provided text (descriptions, messages)
3. **Preserve Debugging Info**: Keep enough information to debug issues
4. **Use Debug Level**: Log detailed information only at debug level

## Setup

Set a unique hash salt in production:

```bash
export LOG_HASH_SALT="your-random-secret-salt-here"
```

Generate a secure salt:
```bash
openssl rand -hex 32
```

## Usage Examples

### Before (Exposes PII)

```go
logger.Log.Debug().Int64("user_id", userID).Msg("Failed to get default currency")

logger.Log.Info().
    Str("description", parsed.Description).
    Str("suggested_category", suggestion.Category).
    Msg("Auto-categorized expense")
```

### After (Privacy-Preserving)

```go
logger.Log.Debug().
    Str("user_hash", logger.HashUserID(userID)).
    Msg("Failed to get default currency")

logger.Log.Info().
    Str("description", logger.SanitizeDescription(parsed.Description)).
    Str("suggested_category", suggestion.Category).
    Msg("Auto-categorized expense")
```

## API Reference

### `HashUserID(userID int64) string`

Creates a consistent 8-character hash of a user ID. Same user always gets the same hash.

```go
logger.Log.Info().
    Str("user_hash", logger.HashUserID(userID)).
    Msg("User action")
```

### `HashChatID(chatID int64) string`

Creates a consistent 8-character hash of a chat ID.

```go
logger.Log.Debug().
    Str("chat_hash", logger.HashChatID(chatID)).
    Msg("Sending message")
```

### `SanitizeDescription(desc string) string`

Redacts expense descriptions but preserves length information.

**Input**: `"lunch at hawker center"`
**Output**: `"<redacted: 4 words, 22 chars>"`

```go
logger.Log.Info().
    Str("description", logger.SanitizeDescription(expense.Description)).
    Msg("Expense created")
```

### `SanitizeText(text string) string`

General-purpose sanitizer for user-provided text.

**Input**: `"coffee"`
**Output**: `"<6 chars>"`

**Input**: `"this is a long message"`
**Output**: `"thi...<22 chars>"`

```go
logger.Log.Debug().
    Str("input", logger.SanitizeText(userMessage)).
    Msg("Processing user input")
```

## What to Hash/Sanitize

### Always Hash
- ✅ User IDs (`userID`, `user.ID`)
- ✅ Chat IDs (`chatID`, `chat.ID`)
- ✅ Any identifier that can track a specific user

### Always Sanitize
- ✅ Expense descriptions
- ✅ User messages
- ✅ Any free-form user input
- ✅ Email addresses, phone numbers, names

### Safe to Log
- ✅ Category names (pre-defined system categories)
- ✅ Currency codes (SGD, USD, etc.)
- ✅ Expense amounts (aggregated/anonymized)
- ✅ Error messages (without user data)
- ✅ Counts and statistics
- ✅ System-generated IDs (expense_id, category_id)

## Complete Example

### Before

```go
func (b *Bot) handleExpense(ctx context.Context, userID int64, chatID int64, desc string) {
    logger.Log.Info().
        Int64("user_id", userID).
        Int64("chat_id", chatID).
        Str("description", desc).
        Msg("Creating expense")

    // ... expense creation logic
}
```

### After

```go
func (b *Bot) handleExpense(ctx context.Context, userID int64, chatID int64, desc string) {
    logger.Log.Info().
        Str("user_hash", logger.HashUserID(userID)).
        Str("chat_hash", logger.HashChatID(chatID)).
        Str("description", logger.SanitizeDescription(desc)).
        Msg("Creating expense")

    // ... expense creation logic
}
```

## Benefits

1. **Privacy**: User identities and content are protected
2. **Debugging**: You can still track patterns by user hash
3. **Compliance**: Helps with GDPR, CCPA, and privacy regulations
4. **Security**: Reduces risk of data exposure in logs
5. **Consistency**: Same user always has same hash within deployment

## Notes

- Hashes are consistent within the same deployment (same salt)
- Changing the salt will change all hashes
- Log rotation and retention policies should still be applied
- Never log passwords, tokens, or API keys (even hashed)
