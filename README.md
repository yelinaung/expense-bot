# Expense Bot

A Telegram bot for tracking personal expenses in SGD (Singapore Dollars) with receipt OCR capabilities using Google Gemini AI.

## Features

- **Quick Expense Tracking**: Add expenses with simple text messages like `5.50 Coffee`
- **Structured Input**: Use commands like `/add 10.50 Lunch Food - Dining Out` for detailed entries
- **Receipt OCR**: Upload receipt photos for automatic expense extraction using Gemini AI
- **Category Management**: Organize expenses with predefined or custom categories
- **Expense Queries**: View expenses by time period (today, this week, recent)
- **Expense Editing**: Modify or delete existing expenses
- **User Whitelisting**: Control who can access your bot
- **Draft Management**: Automatic cleanup of unconfirmed draft expenses
- **Category Caching**: Performance-optimized category lookups

## Architecture

```
expense-bot/
├── cmd/                    # Application entry point
├── internal/
│   ├── bot/               # Telegram bot handlers and logic
│   │   ├── handlers_commands.go    # Command handlers (/start, /add, /list, etc.)
│   │   ├── handlers_receipt.go     # Receipt/photo processing
│   │   ├── handlers_callbacks.go   # Callback query handlers
│   │   ├── parser.go              # Expense input parsing
│   │   └── category_matcher.go    # Smart category matching
│   ├── config/            # Configuration management
│   ├── database/          # Database schema and migrations
│   ├── gemini/            # Google Gemini API integration
│   ├── logger/            # Structured logging
│   ├── models/            # Domain models
│   └── repository/        # Data access layer
├── .gitlab-ci.yml         # CI/CD pipeline
├── Makefile              # Development commands
└── docker-compose.test.yml # Test database setup
```

### Technology Stack

- **Language**: Go 1.21+
- **Database**: PostgreSQL with pgx driver
- **Bot Framework**: go-telegram/bot
- **AI/OCR**: Google Gemini API
- **Testing**: testify, table-driven tests, parallel execution
- **CI/CD**: GitLab CI with linting, SAST, and coverage enforcement

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 14+
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))
- Google Gemini API Key (optional, for receipt OCR)
- Docker and Docker Compose (for testing)

## Installation

### 1. Clone the Repository

```bash
git clone <repository-url>
cd expense-bot
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Set Up Environment Variables

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```bash
# Telegram Bot Token (get from @BotFather)
TELEGRAM_BOT_TOKEN=your_bot_token_here

# PostgreSQL Database Connection
DATABASE_URL=postgres://USER:PASS@localhost:5432/expense_bot?sslmode=disable

# Whitelisted Telegram User IDs (comma-separated)
# Get your user ID by messaging @userinfobot
WHITELISTED_USER_IDS=123456789,987654321

# Gemini API Key (optional - enables receipt OCR)
# Get from https://aistudio.google.com/app/apikey
GEMINI_API_KEY=your_gemini_api_key_here
```

### 4. Set Up Database

Create a PostgreSQL database:

```sql
CREATE DATABASE expense_bot;
```

The bot will automatically run migrations on startup, creating:
- `users` table - Telegram user information
- `categories` table - Expense categories
- `expenses` table - Expense records

Default categories will be seeded automatically.

### 5. Build and Run

```bash
# Build the bot
make build

# Run the bot
./bin/expense-bot
```

Or run directly:

```bash
go run main.go
```

## Usage

### Basic Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/start` | Welcome message and quick start guide | `/start` |
| `/help` | Show all available commands | `/help` |
| `/add <amount> <description> [category]` | Add a structured expense | `/add 5.50 Coffee Food - Dining Out` |
| `/list` | Show recent expenses (last 10) | `/list` |
| `/today` | Show today's expenses with total | `/today` |
| `/week` | Show this week's expenses with total | `/week` |
| `/categories` | List all expense categories | `/categories` |
| `/edit <id> <amount> <description> [category]` | Edit an expense | `/edit 42 6.00 Coffee Food - Dining Out` |
| `/delete <id>` | Delete an expense | `/delete 42` |

### Quick Expense Entry

Simply send a message in the format `<amount> <description> [category]`:

```
5.50 Coffee
10.00 Lunch Food - Dining Out
25 Taxi Transportation
```

The bot will intelligently match category names from your message.

### Receipt OCR

Send a photo of a receipt to automatically extract:
- Amount
- Description/merchant name
- Suggested category (AI-powered)

After extraction, you can:
- ✅ Confirm - Save the expense
- ✏️ Edit - Modify amount, description, or category
- ❌ Cancel - Discard the draft

### Category Matching

The bot uses intelligent category matching:
- Case-insensitive matching
- Partial word matching (e.g., "food" matches "Food - Dining Out")
- Significant word extraction (ignores common words like "the", "a", "and")

## Development

### Available Make Commands

```bash
# Build the application
make build

# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run tests with race detection
make test-race

# Run integration tests (requires Docker)
make test-integration

# Run linter
make lint

# Format code
make fmt

# Clean build artifacts
make clean

# View HTML coverage report
make coverage-html
```

### Running Tests

**Unit tests only:**
```bash
make test
```

**Integration tests with PostgreSQL:**
```bash
make test-integration
```

This will:
1. Start a PostgreSQL container via Docker Compose
2. Run all tests with coverage
3. Generate coverage report
4. Shut down the test database

**Manual integration testing:**
```bash
# Start test database
make test-db-up

# Run tests with TEST_DATABASE_URL set
TEST_DATABASE_URL="postgres://<user>:<pass>@localhost:5433/expense_bot_test?sslmode=disable" go test -v ./...

# Stop test database
make test-db-down
```

### Code Quality

The project uses:
- **golangci-lint** - 28 linters enabled for code quality
- **gofumpt** - Stricter formatting than gofmt
- **Pre-commit hooks** - Automatic formatting, linting, and testing
- **GitLab CI** - Automated testing, SAST, and coverage enforcement (40% minimum)

### Project Standards

- **Error Handling**: All errors wrapped with context using `fmt.Errorf` and `%w`
- **Logging**: Structured logging with zerolog
- **Testing**: Table-driven tests with parallel execution where possible
- **SQL Safety**: All queries use parameterized statements
- **Concurrency**: Proper mutex usage for shared state (`pendingEdits`, `categoryCache`)

## Configuration

### Environment Variables

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `TELEGRAM_BOT_TOKEN` | Yes | Telegram bot API token | - |
| `DATABASE_URL` | Yes | PostgreSQL connection string | - |
| `WHITELISTED_USER_IDS` | Yes | Comma-separated Telegram user IDs | - |
| `GEMINI_API_KEY` | No | Google Gemini API key for OCR | - |

### Bot Configuration

- **Draft Expiration**: 10 minutes (auto-cleanup)
- **Draft Cleanup Interval**: 5 minutes
- **Category Cache TTL**: 5 minutes
- **Currency**: SGD (hardcoded)

## Database Schema

### Users Table
- `id` (BIGINT, PK) - Telegram user ID
- `username`, `first_name`, `last_name` - User info
- `created_at`, `updated_at` - Timestamps

### Categories Table
- `id` (SERIAL, PK) - Category ID
- `name` (TEXT, UNIQUE) - Category name
- `created_at` - Timestamp

### Expenses Table
- `id` (SERIAL, PK) - Expense ID
- `user_id` (BIGINT, FK) - References users
- `amount` (DECIMAL) - Expense amount
- `currency` (TEXT) - Currency code
- `description` (TEXT) - Description
- `category_id` (INT, FK) - References categories
- `receipt_file_id` (TEXT) - Telegram file ID
- `status` (TEXT) - 'draft' or 'confirmed'
- `created_at`, `updated_at` - Timestamps

**Indexes**: user_id, created_at, category_id, status

## Troubleshooting

### Bot not responding

1. Check bot is running: `ps aux | grep expense-bot`
2. Verify token: Test with `curl https://api.telegram.org/bot<TOKEN>/getMe`
3. Check logs for errors
4. Ensure your user ID is in `WHITELISTED_USER_IDS`

### Database connection errors

1. Verify PostgreSQL is running: `psql -U user -d expense_bot`
2. Check `DATABASE_URL` format
3. Ensure database exists and user has permissions

### Receipt OCR not working

1. Verify `GEMINI_API_KEY` is set correctly
2. Check logs for Gemini API errors
3. Ensure image is clear and receipt is visible
4. Check Google AI Studio quota limits

## Contributing

### Development Setup

1. Install pre-commit hooks:
   ```bash
   pip install pre-commit
   pre-commit install
   ```

2. Run tests before committing:
   ```bash
   make test-coverage
   make lint
   ```

### Commit Guidelines

- Fix bugs: Use `/commit` with clear description
- Add features: Create feature branch, test thoroughly
- Follow existing code patterns
- Maintain test coverage above 40%

### Testing Requirements

- Unit tests for all new functions
- Integration tests for database operations
- Table-driven tests for multiple scenarios
- Use `t.Parallel()` where appropriate

## Performance

- **Category Caching**: Categories cached for 5 minutes, reducing database queries
- **Connection Pooling**: pgxpool for efficient PostgreSQL connections
- **Parallel Tests**: Tests run in parallel for faster CI/CD
- **Indexed Queries**: All common queries use database indexes

## Security

- SQL injection prevention via parameterized queries
- User whitelisting for access control
- Secrets detection in CI pipeline
- SAST scanning enabled
- No sensitive data in logs

## Monitoring

The bot uses structured logging with zerolog. All operations log:
- User actions with user_id and username
- Command execution with parameters
- Errors with full context
- Performance metrics (cache hits, query times)

## License

See LICENSE file for details.

## Support

For issues, questions, or contributions, please open an issue in the repository.
