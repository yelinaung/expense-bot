 # Development Guide

## Build/Test/Lint Commands

- **Go version**: 1.25+
- **Build**: `mise build`
- **Test**:
    - `mise run test` for unit tests
    - `mise run test-coverage` for tests with coverage
    - `mise run test-race` to run go tests with race detection
    - `mise run test-integration` to run integration tests against the PostgreSQL test database
- **Lint**:
    - `mise run lint` to run Go vet and golangci-lint
- **Clean**:
    - `mise run clean` to remove build and coverage artifacts
- `grep` is an alias to `rg`.

## Code Style Guidelines

- **Imports**: Use goimports formatting, group stdlib, external, internal packages
- **Formatting**: Use gofumpt (stricter than gofmt), enabled in golangci-lint
- **Naming**: Standard Go conventions - PascalCase for exported, camelCase for unexported
- **Types**: Prefer explicit types, use type aliases for clarity (e.g., `type AgentName string`)
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for wrapping
- **Context**: Always pass context.Context as first parameter for operations
- **Interfaces**: Define interfaces in consuming packages, keep them small and focused
- **Structs**: Use struct embedding for composition, group related fields
- **Constants**: Use typed constants with iota for enums, group in const blocks
- **Testing**: Use testify's `require` package, parallel tests with `t.Parallel()`,
  `t.SetEnv()` to set environment variables. Always use `t.Tempdir()` when in
  need of a temporary directory. This directory does not need to be removed.
- **JSON tags**: Use snake_case for JSON field names
- **File permissions**: Use octal notation (0o755, 0o644) for file permissions
- **Comments**: End comments in periods unless comments are at the end of the line.

ALWAYS RUN these `mise run` commands:
- test
- test-coverage
- test-race
- test-integration

ENSURE that the test coverage stays at or above 50% (CI enforced).

## Test Patterns

### Unit Tests
- Use `t.Parallel()` for tests that don't need database.
- Use table-driven tests for pure functions.
- Use `testify/require` for assertions.
- Use `t.Helper()` in test setup functions.

### Database Tests
- Use `database.TestDB(t)` which skips if `TEST_DATABASE_URL` not set.
- Run with `-p 1` to avoid race conditions.
- Do NOT use `t.Parallel()` for database tests.

### Mocking External Dependencies
- Use interfaces for external SDK calls (e.g., Gemini API).
- Use adapter pattern to wrap SDK structs.
- Create separate constructors for testing (e.g., `NewClientWithGenerator`).
- See `internal/bot/mocks/` for Telegram bot mocks.

### Handler Testing
- Handlers take concrete `*bot.Bot` type, not interface.
- Use wrapper functions to test handler logic without calling real handlers.
- Callback handlers use `EditMessageText` instead of `SendMessage`.

### Edge Cases to Test
- nil/empty slices and maps.
- Whitespace-only inputs.
- Bot mention formats in commands.
- Non-existent IDs for update/delete operations.


## Formatting

- ALWAYS format any Go code you write with `mise fmt`

## Comments

- Comments that live on their own lines should start with capital letters and
  end with periods. Wrap comments at 78 columns.

## Committing

- ALWAYS run both unit and integraton tests before pushing
    - Especially, the fail tests with `mise test-integration 2&>1 | grep -w 'FAIL:'`
- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`, `docs:`, `sec:`, etc).
- ALWAYS run pre-commits before pushing
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.
- Push to all remotes with `mise push-all`.

## Working on the TUI (UI)
Anytime you starts the work, read the AGENTS.md file

Refer to @CLAUDE.md for additional instructions
