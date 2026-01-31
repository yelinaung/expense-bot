# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- **Expense Breakdown Charts**: Generate visual pie charts showing expense distribution by category
  - `/chart week` - Generate weekly expense breakdown chart
  - `/chart month` - Generate monthly expense breakdown chart
  - PNG format with category percentages and legends
  - Includes total expenses, count, and period in caption
  - Uses go-analyze/charts library for chart rendering
- **AI Auto-Categorization**: Automatically categorize expenses using Gemini AI when no category is specified
  - Only applies suggestions with >50% confidence
  - Smart distinction between "Food - Dining Out" and "Food - Grocery"
  - Comprehensive logging for debugging and monitoring
  - Examples: "vegetables" → "Food - Grocery", "taxi" → "Transportation"
- **CSV Report Generation**: Export weekly and monthly expense reports
  - `/report week` - Generate weekly report (Monday-Sunday)
  - `/report month` - Generate monthly report
  - Includes expense details, totals, and counts
  - Filenames include date ranges (e.g., `expenses_month_2026-01.csv`)
- **Enhanced Logging**: Comprehensive debug, info, warn, and error logging throughout
  - SuggestCategory function fully instrumented
  - All Gemini API interactions logged
  - Category matching and confidence scores logged

### Changed
- **Gemini Model**: Updated from `gemini-3-flash-preview` to `gemini-2.5-flash`
- **Token Limits**: Increased MaxOutputTokens from 200 to 500 to prevent response truncation
- **Prompt Optimization**: Simplified and optimized categorization prompt for better efficiency
- **System Instructions**: Added explicit JSON-only instructions to reduce preamble responses

### Fixed
- **Response Truncation**: Fixed bug where Gemini responses were cut off mid-sentence
- **Preamble Handling**: Added `extractJSON()` helper to handle responses with preamble text
- **Monthly Report Filename**: Fixed inconsistent filename format (now uses `expenses_month_` prefix)

## [2026-01-27] - Receipt OCR & Category Matching

### Added
- **Receipt OCR**: Upload receipt photos for automatic expense extraction using Gemini AI
- **Smart Category Matching**: Intelligent category matching with case-insensitive and partial word matching
- **Draft Management**: Automatic cleanup of unconfirmed draft expenses (10-minute expiration)
- **Category Caching**: Performance-optimized category lookups (5-minute cache)

## [2026-01-26] - Initial Release

### Added
- Basic expense tracking via Telegram bot
- PostgreSQL database with migrations
- User whitelisting for access control
- Commands: `/start`, `/help`, `/add`, `/list`, `/today`, `/week`, `/edit`, `/delete`
- Category management with predefined categories
- Quick expense entry with simple text messages
