# Privacy-Preserving Logging

Logs should let you debug without exposing who did what. Four rules get you there:

1. Never log raw user IDs or chat IDs — hash them.
2. Redact or truncate user-provided text (descriptions, messages).
3. Keep enough information to debug issues.
4. Log detailed information only at debug level.

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

Hashed logs still support debugging: the same user always gets the same hash within a deployment, so you can follow one user's activity through the logs without knowing who they are. Redaction also limits what leaks if logs are ever exposed, and it keeps PII out of scope for GDPR/CCPA retention questions.

## Notes

- Hashes are consistent within the same deployment (same salt)
- Changing the salt will change all hashes
- Log rotation and retention policies should still be applied
- Never log passwords, tokens, or API keys (even hashed)
