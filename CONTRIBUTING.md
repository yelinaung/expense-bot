# Contributing to Expense Bot

Thank you for your interest in contributing to Expense Bot!

> **Note**: This project was developed primarily by AI coding agents. Contributions are welcome, but please review the codebase carefully before making changes.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone <your-fork-url>`
3. Trust the project config: `mise trust mise.toml`
4. Install the project tools: `mise install`
5. Install dependencies: `go mod download`
6. Set up pre-commit hooks: `pip install pre-commit && pre-commit install`
7. Copy `.env.example` to `.env` and configure

## Development

### Running Tests

```bash
# Unit tests
mise run test

# With coverage
mise run test-coverage

# Integration tests (requires Docker)
mise run test-integration
```

### Code Quality

```bash
# Format code
mise run fmt

# Run linter
mise run lint
```

## Making Changes

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Make your changes
3. Run tests: `mise run test`
4. Run linter: `mise run lint`
5. Commit with a descriptive message (use [conventional commits](https://www.conventionalcommits.org/))
6. Push and open a pull request

### Commit Message Format

Use semantic commit prefixes:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks
- `sec:` - Security improvements

Example: `fix: handle empty category list in parser`

## Code Style

- Follow existing code patterns
- Use `gofumpt` for formatting
- Add tests for new functionality
- Keep functions small and focused
- Use meaningful variable names

## Reporting Issues

When reporting bugs, please include:
- Go version (`go version`)
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs (redact sensitive info)

## Questions?

Open an issue for questions or discussions.
