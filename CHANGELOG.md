# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.4.0] - 2026-02-08 - Voice Messages & Prompt Injection Mitigation

### Added
- **Voice Message Expense Tracking**: Send a voice message describing your expense (e.g., "spent five fifty on coffee") and Gemini extracts amount, description, currency, and category
- **Shared Sanitization Functions**: Exported `SanitizeForPrompt` and `SanitizeCategoryName` for reuse across parsers
- **Fuzz Testing**: Fuzz test for `SanitizeCategoryName`

### Security
- **Prompt Injection Mitigation**: Sanitize user-created category names before embedding in Gemini prompts
- **Category Name Validation**: `/addcategory` now rejects names with control characters and enforces a 50-character limit
- **Response Field Sanitization**: Sanitize Gemini response fields (merchant, description, category) before storage and display
- **Defensive Prompt Text**: Prompts now instruct Gemini that category lists are data, not instructions

## [v0.3.0] - 2026-02-08

### Added
- **Standalone Category Creation**: `/addcategory <name>` command to create categories directly
- **Per-User Expense Numbering**: Expenses now use per-user sequential numbers instead of global DB IDs
- **Merchant Capture**: Capture merchant name from receipt OCR and user input
- **Inline Edit/Delete Buttons**: Edit and delete buttons on expense confirmations
- **Multi-Currency Support**: Support for multiple currencies with `/currency` and `/setcurrency` commands
- **Username Whitelisting**: Allow whitelisting users by username in addition to user ID
- **Category Filter**: `/category <name>` command to filter expenses by category
- **Fuzz Testing**: Fuzz tests for parsing and sanitization functions
- **Privacy-Preserving Logging**: Hash user IDs in logs for privacy

### Changed
- **Go Version**: Upgraded to Go 1.25.7
- **PostgreSQL**: Updated to PostgreSQL 18

### Fixed
- **Partial Edit Preservation**: `/edit` now preserves unedited fields (description, category)
- **Stale Polling Sessions**: Clear existing webhook/polling sessions on bot startup
- **Prompt Injection**: Hardened category suggestion prompt against injection attacks

### Security
- **Startup Validation**: Added configuration validation on startup
- **Insecure Defaults Audit**: Addressed insecure default configurations

## [v0.2.0] - 2026-01-27 - Receipt OCR & Charts

### Added
- **Expense Breakdown Charts**: Generate visual pie charts showing expense distribution by category
  - `/chart week` - Generate weekly expense breakdown chart
  - `/chart month` - Generate monthly expense breakdown chart
  - PNG format with category percentages and legends
- **AI Auto-Categorization**: Automatically categorize expenses using Gemini AI when no category is specified
  - Only applies suggestions with >50% confidence
  - Smart distinction between "Food - Dining Out" and "Food - Grocery"
- **CSV Report Generation**: Export weekly and monthly expense reports
  - `/report week` - Generate weekly report (Monday-Sunday)
  - `/report month` - Generate monthly report
- **Receipt OCR**: Upload receipt photos for automatic expense extraction using Gemini AI
- **Smart Category Matching**: Intelligent category matching with case-insensitive and partial word matching
- **Draft Management**: Automatic cleanup of unconfirmed draft expenses (10-minute expiration)
- **Category Caching**: Performance-optimized category lookups (5-minute cache)

### Changed
- **Gemini Model**: Updated from `gemini-3-flash-preview` to `gemini-2.5-flash`
- **Token Limits**: Increased MaxOutputTokens from 200 to 500 to prevent response truncation

### Fixed
- **Response Truncation**: Fixed bug where Gemini responses were cut off mid-sentence
- **Monthly Report Filename**: Fixed inconsistent filename format

## [v0.1.0] - 2026-01-26 - Initial Release

### Added
- Basic expense tracking via Telegram bot
- PostgreSQL database with migrations
- User whitelisting for access control
- Commands: `/start`, `/help`, `/add`, `/list`, `/today`, `/week`, `/edit`, `/delete`
- Category management with predefined categories
- Quick expense entry with simple text messages
