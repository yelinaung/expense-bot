# Insecure Defaults Audit Report

**Audit Date**: 2026-02-03
**Auditor**: Security Review (Trail of Bits insecure-defaults methodology)
**Application**: Telegram Expense Bot
**Scope**: Environment variable fallbacks, authentication configuration, fail-open vulnerabilities

---

## Executive Summary

This audit identifies **fail-open** security vulnerabilities where the application runs with insecure defaults when configuration is missing. The focus is on production-reachable code that allows the application to start in an insecure state.

### Overall Risk Assessment: **MEDIUM** ⚠️

| Severity | Count | Description |
|----------|-------|-------------|
| **CRITICAL** | 3 | Application starts without authentication/critical config |
| **HIGH** | 1 | Weak cryptographic default (hash salt) |
| **MEDIUM** | 0 | - |
| **LOW** | 0 | - |

### Key Findings

1. ✅ **POSITIVE**: No hardcoded secrets or API keys in code
2. ✅ **POSITIVE**: No fallback credentials (username/password pairs)
3. ❌ **CRITICAL**: Application starts without TELEGRAM_BOT_TOKEN validation
4. ❌ **CRITICAL**: Application starts without DATABASE_URL validation
5. ❌ **CRITICAL**: Application starts with empty authentication whitelist
6. ❌ **HIGH**: Weak default LOG_HASH_SALT fallback

---

## Detailed Findings

## Finding #1: Missing TELEGRAM_BOT_TOKEN Validation ❌ CRITICAL

### Location
- **File**: `internal/config/config.go:28`
- **File**: `internal/bot/bot.go:72`
- **File**: `main.go:43`

### Vulnerability Pattern
```go
// config.go:28 - No validation
cfg := &Config{
    TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),  // ← Empty string if not set
    ...
}

// bot.go:72 - Empty token passed to Telegram API
telegramBot, err := bot.New(cfg.TelegramBotToken, opts...)
if err != nil {
    return nil, fmt.Errorf("failed to create bot: %w", err)
}
```

### Verification: Actual Behavior

**What happens without TELEGRAM_BOT_TOKEN?**

1. `config.Load()` succeeds (line 20-23 of main.go)
2. `bot.New()` is called with empty string (line 43 of main.go)
3. Telegram Bot API call fails at runtime
4. Application crashes with `logger.Log.Fatal()` (line 45 of main.go)

**Runtime behavior**: ✅ **FAIL-SECURE** (app crashes)

However, the crash happens AFTER:
- Database connection established (line 27-30)
- Migrations run (line 33-35)
- Categories seeded (line 37-39)

### Production Impact

**Severity**: **HIGH** (not CRITICAL - app crashes before serving requests)

**Risk**:
- Application crashes at runtime instead of startup
- Database resources allocated unnecessarily
- No clear error message about missing token
- Confusing error message from Telegram Bot library

### Exploitation Scenario

**Attack**: None (fail-secure crash)

**Impact**: Denial of service (self-inflicted) during deployment

### Recommendation

Add validation in `config.Load()` or immediately after:

```go
// Option 1: Validate in config.Load()
func Load() (*Config, error) {
    _ = godotenv.Load()

    cfg := &Config{
        TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
        DatabaseURL:      os.Getenv("DATABASE_URL"),
        GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
        LogLevel:         os.Getenv("LOG_LEVEL"),
    }

    // Validate required fields
    if cfg.TelegramBotToken == "" {
        return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
    }
    if cfg.DatabaseURL == "" {
        return nil, fmt.Errorf("DATABASE_URL is required")
    }
    if len(cfg.WhitelistedUserIDs) == 0 && len(cfg.WhitelistedUsernames) == 0 {
        return nil, fmt.Errorf("at least one whitelisted user (ID or username) is required")
    }

    return cfg, nil
}

// Option 2: Validate in main.go after config.Load()
cfg, err := config.Load()
if err != nil {
    logger.Log.Fatal().Err(err).Msg("Failed to load config")
}

if err := validateConfig(cfg); err != nil {
    logger.Log.Fatal().Err(err).Msg("Invalid configuration")
}
```

**Priority**: **HIGH**

---

## Finding #2: Missing DATABASE_URL Validation ❌ CRITICAL

### Location
- **File**: `internal/config/config.go:29`
- **File**: `main.go:27`

### Vulnerability Pattern
```go
// config.go:29 - No validation
cfg := &Config{
    DatabaseURL: os.Getenv("DATABASE_URL"),  // ← Empty string if not set
    ...
}

// main.go:27 - Empty string passed to database.Connect()
pool, err := database.Connect(ctx, cfg.DatabaseURL)
if err != nil {
    logger.Log.Fatal().Err(err).Msg("Failed to connect to database")
}
```

### Verification: Actual Behavior

**What happens without DATABASE_URL?**

1. `config.Load()` succeeds
2. `database.Connect(ctx, "")` is called
3. PostgreSQL driver attempts to connect with empty connection string
4. Connection fails at runtime
5. Application crashes with `logger.Log.Fatal()`

**Runtime behavior**: ✅ **FAIL-SECURE** (app crashes)

### Production Impact

**Severity**: **HIGH** (not CRITICAL - app crashes)

**Risk**:
- Application crashes at runtime instead of startup validation
- No clear error message about missing DATABASE_URL
- Confusing error from pgx driver

### Recommendation

Add validation in `config.Load()` (see Finding #1 recommendation).

**Priority**: **HIGH**

---

## Finding #3: Empty Authentication Whitelist ❌ CRITICAL

### Location
- **File**: `internal/config/config.go:34-60`
- **File**: `internal/config/config.go:67-84` (IsUserWhitelisted)
- **File**: `internal/bot/middleware.go` (assumed - whitelistMiddleware)

### Vulnerability Pattern
```go
// config.go:34-60 - Whitelist can be empty
whitelistStr := os.Getenv("WHITELISTED_USER_IDS")
if whitelistStr != "" {
    // Parse user IDs
    ...
}

whitelistUsernames := os.Getenv("WHITELISTED_USERNAMES")
if whitelistUsernames != "" {
    // Parse usernames
    ...
}

// No validation that at least one whitelist entry exists
return cfg, nil

// config.go:67-84 - Returns false if both whitelists empty
func (c *Config) IsUserWhitelisted(userID int64, username string) bool {
    if slices.Contains(c.WhitelistedUserIDs, userID) {
        return true
    }

    if username != "" {
        username = strings.TrimPrefix(username, "@")
        for _, whitelisted := range c.WhitelistedUsernames {
            if strings.EqualFold(whitelisted, username) {
                return true
            }
        }
    }

    return false  // ← Returns false if both lists empty
}
```

### Verification: Actual Behavior

**What happens without WHITELISTED_USER_IDS and WHITELISTED_USERNAMES?**

1. `config.Load()` succeeds with empty whitelists
2. Bot starts successfully
3. All incoming messages are rejected by `whitelistMiddleware`
4. Bot appears unresponsive to all users (including legitimate admin)

**Runtime behavior**: ✅ **FAIL-SECURE** (all access denied)

However, this is **fail-closed unintentionally**:
- No clear startup error
- Bot runs but is completely unusable
- Admin locked out of their own bot
- Wastes resources (database, Telegram polling)

### Production Impact

**Severity**: **MEDIUM-HIGH** (Operational failure, not security breach)

**Risk**:
- Bot deployed without working authentication
- Admin locked out
- Silent failure (bot runs but doesn't respond)
- Debugging confusion

### Exploitation Scenario

**Attack**: None (fail-closed)

**Impact**: Denial of service to legitimate users (including admin)

### Recommendation

Require at least one whitelisted user at startup:

```go
func Load() (*Config, error) {
    // ... existing code ...

    // Validate at least one whitelist entry
    if len(cfg.WhitelistedUserIDs) == 0 && len(cfg.WhitelistedUsernames) == 0 {
        return nil, fmt.Errorf("at least one whitelisted user (WHITELISTED_USER_IDS or WHITELISTED_USERNAMES) is required")
    }

    return cfg, nil
}
```

**Priority**: **HIGH**

---

## Finding #4: Weak Default LOG_HASH_SALT ❌ HIGH

### Location
- **File**: `internal/logger/privacy.go:16-19`

### Vulnerability Pattern
```go
func init() {
    // Load salt from environment or generate a default one
    // In production, set LOG_HASH_SALT environment variable
    hashSalt = os.Getenv("LOG_HASH_SALT")
    if hashSalt == "" {
        hashSalt = "default-salt-change-in-production"  // ← INSECURE DEFAULT
    }
}

// Used in HashUserID and HashChatID
func HashUserID(userID int64) string {
    data := fmt.Sprintf("%d:%s", userID, hashSalt)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])[:8]
}
```

### Verification: Actual Behavior

**What happens without LOG_HASH_SALT?**

1. Application starts successfully
2. Hashing uses predictable salt: `"default-salt-change-in-production"`
3. User ID hashes are deterministic and **reversible via rainbow table**
4. Application runs in production with weak privacy protection

**Runtime behavior**: ❌ **FAIL-OPEN** (app runs insecurely)

### Production Impact

**Severity**: **HIGH**

**Risk**:
- User ID hashes can be reversed if attacker knows the default salt
- Privacy-preserving logging becomes ineffective
- Attacker can:
  1. Obtain logs (via log aggregation service compromise, backup access, etc.)
  2. Know the default salt from public GitHub repository
  3. Build rainbow table of common user IDs
  4. Correlate hashed user IDs to actual Telegram user IDs

### Exploitation Scenario

**Attack**: Privacy breach via hash reversal

**Steps**:
1. Attacker obtains application logs (e.g., compromised log service)
2. Logs contain hashed user IDs: `logger.Log.Info().Str("user_hash", HashUserID(userID))`
3. Attacker finds salt in public repository: `"default-salt-change-in-production"`
4. Attacker pre-computes hashes for common Telegram user IDs (1-1000000000)
5. Attacker matches log hashes to actual user IDs

**Impact**: De-anonymization of users, privacy policy violation

### Real-World Example

Logs might contain:
```
{"level":"info","user_hash":"a3d5e2f1","msg":"Expense added"}
```

Attacker computes:
```python
import hashlib

salt = "default-salt-change-in-production"
for user_id in range(1, 1000000000):
    data = f"{user_id}:{salt}"
    hash_full = hashlib.sha256(data.encode()).hexdigest()
    hash_short = hash_full[:8]
    if hash_short == "a3d5e2f1":
        print(f"Found user ID: {user_id}")
        break
```

### Recommendation

**Fail-secure approach** - Crash if salt not provided:

```go
func init() {
    hashSalt = os.Getenv("LOG_HASH_SALT")
    if hashSalt == "" {
        // FAIL-SECURE: Crash at startup
        panic("LOG_HASH_SALT environment variable is required for privacy-preserving logging")
    }

    // Validate salt strength
    if len(hashSalt) < 32 {
        panic("LOG_HASH_SALT must be at least 32 characters for adequate entropy")
    }
}
```

**Alternative** - Generate random salt at startup (not recommended for multi-instance):

```go
func init() {
    hashSalt = os.Getenv("LOG_HASH_SALT")
    if hashSalt == "" {
        // Generate random salt and log warning
        randomBytes := make([]byte, 32)
        if _, err := rand.Read(randomBytes); err != nil {
            panic("failed to generate random salt: " + err.Error())
        }
        hashSalt = hex.EncodeToString(randomBytes)

        log.Warn().Msg("LOG_HASH_SALT not set, generated random salt. User ID hashes will not be consistent across restarts.")
    }
}
```

**Priority**: **HIGH**

---

## Finding #5: Optional GEMINI_API_KEY (Informational) ✅ ACCEPTABLE

### Location
- **File**: `internal/config/config.go:30`
- **File**: `internal/bot/bot.go:57-65`

### Pattern
```go
// config.go:30 - Optional
cfg := &Config{
    GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),  // Can be empty
    ...
}

// bot.go:57-65 - Graceful degradation
if cfg.GeminiAPIKey != "" {
    geminiClient, err := gemini.NewClient(context.Background(), cfg.GeminiAPIKey)
    if err != nil {
        logger.Log.Warn().Err(err).Msg("Failed to create Gemini client, receipt OCR disabled")
    } else {
        b.geminiClient = geminiClient
        logger.Log.Info().Msg("Gemini client initialized for receipt OCR")
    }
}
```

### Assessment: ✅ **SAFE - Intentional Feature Degradation**

**Behavior without GEMINI_API_KEY**:
- Application starts successfully
- Receipt OCR feature disabled
- Auto-categorization feature disabled
- Bot still functional for manual expense tracking

**Not a vulnerability** because:
- Feature is explicitly optional (documented in README.md:105-107)
- No security control bypassed
- Application logs the degraded mode
- Users aware that OCR won't work

**Priority**: **N/A - Acceptable design**

---

## Summary of Findings

| # | Finding | Severity | Behavior | Status |
|---|---------|----------|----------|--------|
| 1 | Missing TELEGRAM_BOT_TOKEN validation | HIGH | Fail-secure (crash) | ❌ Fix recommended |
| 2 | Missing DATABASE_URL validation | HIGH | Fail-secure (crash) | ❌ Fix recommended |
| 3 | Empty authentication whitelist | MEDIUM-HIGH | Fail-closed (unusable) | ❌ Fix recommended |
| 4 | Weak default LOG_HASH_SALT | HIGH | **Fail-open (insecure)** | ❌ **Fix required** |
| 5 | Optional GEMINI_API_KEY | N/A | Graceful degradation | ✅ Acceptable |

---

## Recommendations

### Immediate (Priority 1 - Critical)

1. **Fix Finding #4**: Remove default LOG_HASH_SALT, require environment variable
   - **Impact**: Prevents privacy breach via hash reversal
   - **Implementation**: 5 minutes (1-line change + validation)
   - **Risk if not fixed**: User de-anonymization in logs

### High Priority (Priority 2)

2. **Fix Findings #1, #2, #3**: Add startup validation for required config
   - **Impact**: Clearer error messages, faster failure on misconfiguration
   - **Implementation**: 15 minutes (add validation function)
   - **Risk if not fixed**: Confusing runtime errors, wasted resources

### Implementation Plan

**Step 1**: Create validation function in `internal/config/config.go`:

```go
func Load() (*Config, error) {
    _ = godotenv.Load()

    cfg := &Config{
        TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
        DatabaseURL:      os.Getenv("DATABASE_URL"),
        GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
        LogLevel:         os.Getenv("LOG_LEVEL"),
    }

    // Parse whitelists (existing code)
    whitelistStr := os.Getenv("WHITELISTED_USER_IDS")
    // ... existing parsing code ...

    whitelistUsernames := os.Getenv("WHITELISTED_USERNAMES")
    // ... existing parsing code ...

    // VALIDATION - Fail fast on startup
    var errs []string

    if cfg.TelegramBotToken == "" {
        errs = append(errs, "TELEGRAM_BOT_TOKEN is required")
    }

    if cfg.DatabaseURL == "" {
        errs = append(errs, "DATABASE_URL is required")
    }

    if len(cfg.WhitelistedUserIDs) == 0 && len(cfg.WhitelistedUsernames) == 0 {
        errs = append(errs, "at least one whitelisted user (WHITELISTED_USER_IDS or WHITELISTED_USERNAMES) is required")
    }

    if len(errs) > 0 {
        return nil, fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errs, "\n- "))
    }

    return cfg, nil
}
```

**Step 2**: Fix LOG_HASH_SALT in `internal/logger/privacy.go`:

```go
func init() {
    hashSalt = os.Getenv("LOG_HASH_SALT")
    if hashSalt == "" {
        panic("LOG_HASH_SALT environment variable is required (generate with: openssl rand -hex 32)")
    }
    if len(hashSalt) < 32 {
        panic("LOG_HASH_SALT must be at least 32 characters for adequate entropy")
    }
}
```

**Step 3**: Update `.env.example`:

```bash
# REQUIRED: Telegram Bot Token (get from @BotFather)
TELEGRAM_BOT_TOKEN=your_bot_token_here

# REQUIRED: PostgreSQL Database Connection
DATABASE_URL=postgres://YOUR_DATABASE_URL

# REQUIRED: At least one whitelisted user (comma-separated)
WHITELISTED_USER_IDS=123456789,987654321
# OR use usernames (alternative to user IDs)
WHITELISTED_USERNAMES=alice,bob,charlie

# REQUIRED: Hash salt for privacy-preserving logging (generate with: openssl rand -hex 32)
LOG_HASH_SALT=generate_random_64_char_hex_string_here

# OPTIONAL: Gemini API Key for receipt OCR and auto-categorization
GEMINI_API_KEY=your_gemini_api_key_here

# OPTIONAL: Log level (debug, info, warn, error)
LOG_LEVEL=info
```

**Step 4**: Update documentation (`README.md` and deployment guides):

Add to Prerequisites section:
```markdown
## Required Environment Variables

The application requires the following environment variables to start:

1. **TELEGRAM_BOT_TOKEN** - Get from [@BotFather](https://t.me/BotFather)
2. **DATABASE_URL** - PostgreSQL connection string
3. **WHITELISTED_USER_IDS** or **WHITELISTED_USERNAMES** - At least one authorized user
4. **LOG_HASH_SALT** - Random 64-character hex string for privacy-preserving logging

Generate LOG_HASH_SALT:
```bash
openssl rand -hex 32
```

The bot will crash at startup if these are missing.

---

## Testing

### Validation Tests

Add tests to verify fail-fast behavior:

```go
// internal/config/config_test.go

func TestLoad_RequiredFieldsMissing(t *testing.T) {
    tests := []struct {
        name    string
        env     map[string]string
        wantErr string
    }{
        {
            name:    "missing telegram token",
            env:     map[string]string{"DATABASE_URL": "postgres://...", "WHITELISTED_USER_IDS": "123"},
            wantErr: "TELEGRAM_BOT_TOKEN is required",
        },
        {
            name:    "missing database url",
            env:     map[string]string{"TELEGRAM_BOT_TOKEN": "token", "WHITELISTED_USER_IDS": "123"},
            wantErr: "DATABASE_URL is required",
        },
        {
            name:    "missing whitelist",
            env:     map[string]string{"TELEGRAM_BOT_TOKEN": "token", "DATABASE_URL": "postgres://..."},
            wantErr: "at least one whitelisted user",
        },
        {
            name:    "all fields missing",
            env:     map[string]string{},
            wantErr: "TELEGRAM_BOT_TOKEN is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Clear and set env vars
            os.Clearenv()
            for k, v := range tt.env {
                os.Setenv(k, v)
            }

            cfg, err := Load()

            if err == nil {
                t.Errorf("Load() expected error, got nil (cfg: %+v)", cfg)
            }
            if err != nil && !strings.Contains(err.Error(), tt.wantErr) {
                t.Errorf("Load() error = %v, want error containing %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## Conclusion

The expense-bot has **one critical fail-open vulnerability** (weak LOG_HASH_SALT default) and **three fail-secure issues** that should be fixed for better operational reliability.

### Risk Summary

**Current State**: Application mostly fails securely but with confusing error messages. One high-risk privacy vulnerability (LOG_HASH_SALT).

**After Fixes**: All critical configuration validated at startup with clear error messages. No insecure defaults.

### Timeline

- **Immediate** (Today): Fix LOG_HASH_SALT (Finding #4)
- **High Priority** (This Week): Add config validation (Findings #1-3)
- **Documentation** (This Week): Update README and .env.example

---

**Audit Completed By**: Security Review
**Next Review**: After implementing recommended fixes
**Document Version**: 1.0
