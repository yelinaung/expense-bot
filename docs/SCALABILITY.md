# Scalability Analysis - Expense Tracker Bot

## Current Implementation: Single-Instance Only ❌

**You are correct**: You **cannot run multiple instances** of this bot with the current implementation.

## Why Multiple Instances Won't Work

### 1. **Telegram API Limitation** 🚫 (CRITICAL)

**Problem**: Telegram Bot API only allows **ONE active connection per bot token**.

```
Instance 1 → polls Telegram → gets updates ✅
Instance 2 → polls Telegram → conflicts with Instance 1 ❌
```

**What happens if you try:**
- **Long Polling (current)**: Only one instance receives updates (non-deterministic which one)
- Updates may be lost or duplicated
- Telegram may return errors: `409 Conflict: terminated by other getUpdates request`

**Solution**: Switch from long polling to webhooks (see "How to Scale" section below)

### 2. **In-Memory State** 🧠 (HIGH IMPACT)

**Problem**: User edit state is stored in RAM, not shared between instances.

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

### 3. **Category Cache** 💾 (MEDIUM IMPACT)

**Problem**: Categories cached in each instance's memory.

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

### 4. **Draft Cleanup Race Conditions** ⏰ (LOW IMPACT)

**Problem**: Each instance runs its own cleanup loop.

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

### Vertical Scaling (Single Instance) ✅

**Current capacity** (estimated):

| Metric | Limit | Bottleneck |
|--------|-------|------------|
| Concurrent users | ~1,000-5,000 | CPU-bound (Telegram API calls) |
| Requests/second | ~100-500 | Network I/O, Gemini API |
| Database connections | 25 (default pool) | PostgreSQL connection limit |
| Memory usage | ~50-200 MB | Small footprint |
| Receipt OCR throughput | ~1-2/second | Gemini API rate limits |

**Good for**: Small to medium deployments (hundreds to low thousands of users)

**When you'll need to scale:**
- \>5,000 active users
- \>100 receipts/minute
- Global user base (latency concerns)

### Horizontal Scaling (Multiple Instances) ❌

**Current status**: NOT POSSIBLE without major refactoring

## How to Scale (Options)

### Option 1: Optimize Single Instance (Easiest) ✅

**Recommended for <10,000 users**

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

### Option 2: Webhook + Load Balancer (Moderate Difficulty) ⚙️

**Required for >10,000 users**

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

### Option 3: Message Queue Architecture (Hard) 🏗️

**For enterprise scale (>100,000 users)**

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

### Current Schema (Scalable ✅)

**Good design choices:**
- ✅ Proper indexes on foreign keys and date columns
- ✅ Normalized schema (no duplicate data)
- ✅ BIGINT for user IDs (Telegram can have large IDs)
- ✅ DECIMAL for amounts (financial accuracy)

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
- **Paid tier**: 1,000 requests/minute (Gemini 1.5 Flash)
- **Timeout**: 30 seconds per request (you set to 30s)
- **Image size**: 20MB max

**Bottleneck**: Gemini will become a bottleneck before anything else

**Solution**:
```go
// Implement request queue for Gemini
type GeminiQueue struct {
    semaphore chan struct{} // Limit concurrent requests
    rateLimit *rate.Limiter  // 50 requests/minute
}

// Queue receipt processing
queue.Process(imageBytes) // Blocks if at limit
```

## Monitoring & Metrics (Not Implemented)

**What you should add:**

```go
// Prometheus metrics
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "bot_requests_total"},
        []string{"command"},
    )

    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{Name: "bot_request_duration_seconds"},
        []string{"command"},
    )

    activeUsers = prometheus.NewGauge(
        prometheus.GaugeOpts{Name: "bot_active_users"},
    )

    geminiQueueDepth = prometheus.NewGauge(
        prometheus.GaugeOpts{Name: "gemini_queue_depth"},
    )
)
```

**Observability:**
- Request latency per command
- Error rates
- Database connection pool usage
- Cache hit/miss ratios
- Gemini API response times
- Active user count

## Recommendations

### If you have <1,000 users: ✅ No changes needed
- Current single-instance setup is fine
- Focus on features, not scalability

### If you have 1,000-10,000 users: ⚙️ Minor optimizations
1. Add Redis for category cache
2. Increase database connection pool
3. Add monitoring (Prometheus + Grafana)
4. Set up database backups

### If you have >10,000 users: 🔧 Major refactoring needed
1. Switch to webhooks
2. Move state to Redis
3. Deploy multiple bot instances
4. Set up load balancer
5. Add read replicas for database
6. Implement Gemini request queue
7. Add comprehensive monitoring

### If you plan for >100,000 users: 🏗️ Rearchitect
1. Message queue architecture (RabbitMQ/Kafka)
2. Worker pool for processing
3. Database sharding
4. CDN for static assets (if you add a web UI)
5. Multi-region deployment
6. Dedicated ops team

## Quick Scalability Checklist

- [ ] Database indexes on user_id, created_at, category_id ✅ (Already done)
- [ ] Connection pooling configured ✅ (Already done)
- [ ] Monitoring and alerting ❌ (Not implemented)
- [ ] Database backups ❌ (Not verified)
- [ ] Rate limiting per user ❌ (Only whitelist exists)
- [ ] Redis for shared state ❌ (Not implemented)
- [ ] Webhook mode ❌ (Currently polling)
- [ ] Load testing ❌ (Never done)
- [ ] Disaster recovery plan ❌ (Not documented)
- [ ] Auto-scaling policies ❌ (Single instance)

## Conclusion

**Current state**: Single-instance, good for small deployments

**Can you run multiple instances?**: **NO** ❌
- Telegram API limitation (polling conflict)
- In-memory state (pendingEdits)
- No shared cache

**When to scale**: When you hit 5,000-10,000 active users or 50+ receipts/minute

**Easiest scaling path**:
1. Add monitoring (know when you need to scale)
2. Switch to webhooks (enables multiple instances)
3. Add Redis (share state across instances)
4. Deploy behind load balancer
5. Add read replicas (database scaling)

**Good news**: Your database schema and code structure are well-designed for scaling. Most changes would be infrastructure (Redis, webhooks, load balancer) rather than code rewrites.
