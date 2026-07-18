# Scalability Analysis - Expense Tracker Bot

The bot is single-instance by design. Start a second instance against the same token and things break in four ways, listed here from critical to cosmetic.

## Why Multiple Instances Won't Work

### 1. Telegram API Limitation (CRITICAL)

The Telegram Bot API allows one active `getUpdates` connection per bot token.

```
Instance 1 → polls Telegram → gets updates ✅
Instance 2 → polls Telegram → conflicts with Instance 1 ❌
```

With two instances polling, only one receives each update — and which one is non-deterministic. Updates get lost or duplicated, and Telegram returns `409 Conflict: terminated by other getUpdates request`.

**Solution**: Switch from long polling to webhooks (see "How to Scale" below)

### 2. In-Memory State (HIGH IMPACT)

User edit state lives in RAM, invisible to other instances.

```go
// Line 38-39 in bot.go
pendingEdits   map[int64]*pendingEdit // ❌ In-memory map
pendingEditsMu sync.RWMutex
```

**Example failure:**
```
User sends: /edit 123
  → Instance 1: stores edit state in memory
User sends: 50.00
  → Instance 2: receives message, doesn't know about edit state
  → ❌ Edit fails, user confused
```

**Impact**: Edit amount/category features break randomly

**Solution**: Move `pendingEdits` to Redis or database

### 3. Category Cache (MEDIUM IMPACT)

Each instance caches categories in its own memory.

```go
// Line 42-44 in bot.go
categoryCache       []models.Category  // ❌ Per-instance cache
categoryCacheExpiry time.Time
categoryCacheMu     sync.RWMutex
```

**Impact**:
- Cache invalidation doesn't propagate across instances
- Higher database load (each instance caches separately)
- Not critical (just less efficient)

**Solution**: Use Redis for shared cache

### 4. Draft Cleanup Race Conditions (LOW IMPACT)

Each instance runs its own cleanup loop.

```go
// Line 96 in bot.go
go b.startDraftCleanupLoop(ctx)  // ❌ Each instance runs this
```

**What happens:**
- All instances try to delete expired drafts
- Multiple DELETE queries for same drafts
- Database handles this OK (idempotent), but wasteful

**Impact**: Increased database load, but functionally harmless

**Solution**: Use distributed locks or designate one instance as cleanup leader

## Current Scalability Limits

### Vertical Scaling (Single Instance)

Estimated capacity of one instance:

| Metric | Limit | Bottleneck |
|--------|-------|------------|
| Concurrent users | ~1,000-5,000 | CPU-bound (Telegram API calls) |
| Requests/second | ~100-500 | Network I/O, Gemini API |
| Database connections | 25 (default pool) | PostgreSQL connection limit |
| Memory usage | ~50-200 MB | Small footprint |
| Receipt OCR throughput | ~1-2/second | Gemini API rate limits |

That covers small to medium deployments — hundreds to low thousands of users. Plan to scale past it when you cross 5,000 active users, 100 receipts/minute, or pick up a global user base with latency concerns.

### Horizontal Scaling (Multiple Instances)

Not possible without the refactoring described below.

## How to Scale (Options)

### Option 1: Optimize Single Instance (Easiest)

Recommended for under 10,000 users.

**1. Increase Database Connection Pool**
```go
// In database.Connect()
config.MaxConns = 50  // Default is 25
config.MinConns = 10
```

**2. Add Database Indexes** (already done ✅)
```sql
CREATE INDEX idx_expenses_user_id ON expenses(user_id);
CREATE INDEX idx_expenses_created_at ON expenses(created_at);
```

**3. Scale PostgreSQL Vertically**
- Increase CPU/RAM on database server
- Use read replicas for /list, /today, /week queries
- Keep writes on primary

**4. Add Redis Cache for Categories**
```go
// Cache categories in Redis with 5-minute TTL
// Reduces DB load for frequent queries
```

**5. Rate Limit Users**
```go
// Prevent abuse: max 10 requests/minute per user
// Already have whitelist, add rate limiter
```

**Estimated capacity**: Up to 10,000-20,000 users

### Option 2: Webhook + Load Balancer (Moderate Difficulty)

Required past 10,000 users.

**Architecture:**
```
           Telegram
              │
              ↓ (webhook)
         Load Balancer
         /    |    \
    Bot-1  Bot-2  Bot-3  (multiple instances)
         \    |    /
              ↓
         PostgreSQL
              +
            Redis
```

**Changes needed:**

**1. Switch to Webhooks**
```go
// Replace bot.Start(ctx) with:
bot.StartWebhook(ctx, webhookConfig)

// In main.go:
http.HandleFunc("/webhook", botInstance.HandleWebhook)
http.ListenAndServe(":8080", nil)
```

**2. Move State to Redis**
```go
// Replace pendingEdits map with Redis:
type Bot struct {
    redisClient *redis.Client  // Shared state
    // Remove: pendingEdits map
}

// Store edit state:
redis.Set(ctx, fmt.Sprintf("edit:%d", chatID), editJSON, 10*time.Minute)

// Retrieve edit state:
redis.Get(ctx, fmt.Sprintf("edit:%d", chatID))
```

**3. Shared Category Cache (Redis)**
```go
// Cache categories in Redis instead of memory
redis.Set(ctx, "categories", categoriesJSON, 5*time.Minute)
```

**4. Distributed Cleanup (Redis Lock)**
```go
// Only one instance runs cleanup at a time
func (b *Bot) startDraftCleanupLoop(ctx context.Context) {
    for {
        // Try to acquire lock
        acquired := redis.SetNX(ctx, "cleanup:lock", "1", 5*time.Minute)
        if acquired {
            b.cleanupExpiredDrafts(ctx)
        }
        time.Sleep(5 * time.Minute)
    }
}
```

**5. Stateless Bot Instances**
```
Each instance should be identical
Environment variables: DB_URL, REDIS_URL, BOT_TOKEN
No local state (everything in DB/Redis)
```

**Estimated capacity**: Up to 100,000+ users

**Infrastructure:**
- 3-5 bot instances (auto-scaling)
- 1 PostgreSQL primary + 2 read replicas
- 1 Redis cluster (HA setup)
- 1 load balancer (NGINX, HAProxy, or cloud LB)

### Option 3: Message Queue Architecture (Hard)

For enterprise scale, past 100,000 users.

**Architecture:**
```
Telegram → Webhook Handler → RabbitMQ/Kafka
                                   ↓
                            Worker Pool (10-100 workers)
                                   ↓
                           Database + Redis
```

**Benefits:**
- Decoupled processing
- Easy to scale workers independently
- Better error handling and retries
- Rate limiting at queue level

**Drawbacks:**
- Complex infrastructure
- More moving parts
- Higher operational overhead

## Database Scalability

### Current Schema

The schema already scales well:
- Indexes on foreign keys and date columns
- Normalized, no duplicate data
- BIGINT for user IDs (Telegram IDs can be large)
- DECIMAL for amounts (financial accuracy)

**Potential bottlenecks:**

**1. Large Table Scans**
```sql
-- This query gets slower as expenses table grows:
SELECT * FROM expenses WHERE user_id = ? ORDER BY created_at DESC;

-- Solution: Already indexed ✅
CREATE INDEX idx_expenses_user_id ON expenses(user_id);
CREATE INDEX idx_expenses_created_at ON expenses(created_at);
```

**2. Category Cache Miss Storm**
```
All instances restart → all cache empty → 1000 requests to DB at once

-- Solution: Implement cache warming on startup
```

**3. Connection Pool Exhaustion**
```
Under load: 25 connections not enough
-- Solution: Increase max_connections in PostgreSQL
```

### Scaling Strategies

**Vertical (Single Server)**
- ✅ Current setup works fine
- Can handle 10,000-50,000 users
- Upgrade to larger PostgreSQL instance

**Horizontal (Sharding)**
- Only needed at >100,000 users
- Shard by user_id (modulo sharding)
- Each shard handles subset of users
- Complex, avoid unless necessary

**Read Replicas**
- Needed at >20,000 active users
- Route reads to replicas:
  - `/list`, `/today`, `/week` → replica
  - `/add`, `/delete`, `/edit` → primary
- Reduces load on primary

## Third-Party Service Limits

### Telegram Bot API
- **Rate limit**: ~30 messages/second to same chat
- **Rate limit**: ~1 message/second across different chats (unofficial)
- **File downloads**: No explicit limit, but throttled
- **Webhook**: 100 connections max

### Google Gemini API
- **Free tier**: 60 requests/minute
- **Paid tier**: 1,000 requests/minute (gemini-2.5-flash, the model in use)
- **Timeout**: 30 seconds per request (you set to 30s)
- **Image size**: 20MB max

Gemini hits its limit before anything else does. When it does:
```go
// Implement request queue for Gemini
type GeminiQueue struct {
    semaphore chan struct{} // Limit concurrent requests
    rateLimit *rate.Limiter  // 50 requests/minute
}

// Queue receipt processing
queue.Process(imageBytes) // Blocks if at limit
```

## Monitoring & Metrics

OpenTelemetry tracing and metrics are implemented — set `OTEL_ENABLED=true` to export to any OTLP backend (see [OTEL_INTEGRATION.md](./OTEL_INTEGRATION.md)). Covered today:

- Handler counts, durations, and in-flight requests per command
- Expense CRUD operations and amounts
- External API durations and errors (Gemini, Frankfurter, Telegram)
- Background job runs and durations
- Cache hit/miss ratios
- Database query spans via otelpgx

Still missing: alerting rules and dashboards. Build those in your backend (Grafana, Datadog) on top of the exported metrics.

## Recommendations

### Under 1,000 users: no changes
The single-instance setup is fine. Spend the time on features.

### 1,000–10,000 users: minor optimizations
1. Add Redis for category cache
2. Increase database connection pool
3. Enable OTel export and build dashboards/alerts on it
4. Set up database backups

### Over 10,000 users: major refactoring
1. Switch to webhooks
2. Move state to Redis
3. Deploy multiple bot instances
4. Set up load balancer
5. Add read replicas for database
6. Implement Gemini request queue
7. Add comprehensive monitoring

### Over 100,000 users: rearchitect
1. Message queue architecture (RabbitMQ/Kafka)
2. Worker pool for processing
3. Database sharding
4. CDN for static assets (if you add a web UI)
5. Multi-region deployment
6. Dedicated ops team

## Quick Scalability Checklist

- [ ] Database indexes on user_id, created_at, category_id ✅ (Already done)
- [ ] Connection pooling configured ✅ (Already done)
- [x] Metrics and tracing ✅ (OpenTelemetry, opt-in via `OTEL_ENABLED`)
- [ ] Alerting and dashboards ❌ (Backend-side, not set up)
- [ ] Database backups ❌ (Not verified)
- [ ] Rate limiting per user ❌ (Only whitelist exists)
- [ ] Redis for shared state ❌ (Not implemented)
- [ ] Webhook mode ❌ (Currently polling)
- [ ] Load testing ❌ (Never done)
- [ ] Disaster recovery plan ❌ (Not documented)
- [ ] Auto-scaling policies ❌ (Single instance)

## Conclusion

The bot runs single-instance and serves small deployments well. Multiple instances fail today on three counts: the polling conflict with Telegram, in-memory `pendingEdits` state, and per-instance caches. Scaling becomes a real concern around 5,000–10,000 active users or 50+ receipts/minute.

The path, in order:
1. Enable OTel export and alerting, so you know when you need to scale
2. Switch to webhooks, which unlocks multiple instances
3. Add Redis to share state across instances
4. Deploy behind a load balancer
5. Add read replicas for the database

The schema and code structure are already built for this — the work is infrastructure (Redis, webhooks, load balancer), not code rewrites.
