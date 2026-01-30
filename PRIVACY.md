# Privacy Policy - Expense Tracker Bot

## Data Collection

This bot collects and processes the following information:

### User Data
- Telegram User ID (numeric identifier)
- Username, First Name, Last Name (from Telegram profile)
- Expense records (amounts, descriptions, categories, dates)

### Receipt Photos
When you send a receipt photo:
1. **Photo is NOT stored permanently on our servers**
2. Photo is temporarily downloaded to server memory (RAM) for processing
3. Photo is sent to Google Gemini AI for optical character recognition (OCR)
4. Only extracted data (amount, merchant, category) is saved to our database
5. A Telegram file reference ID is stored to allow viewing the original receipt via Telegram

## Third-Party Services

### Telegram
- All communication happens through Telegram's infrastructure
- Photos remain stored on Telegram's servers indefinitely
- Governed by [Telegram Privacy Policy](https://telegram.org/privacy)
- We can retrieve your photos using Telegram's file ID

### Google Gemini AI
- Receipt photos are sent to Google's Gemini API for text extraction
- Google may retain data according to their [Gemini Privacy Notice](https://support.google.com/gemini/answer/13594961)
- Photos may be used to improve AI models (check your Google account settings)
- Processing typically takes 1-3 seconds

## Data Storage

### Our Database (PostgreSQL)
We store:
- User profile information (ID, username, name)
- Expense records (amount, description, category, date, status)
- Telegram file IDs (references to photos on Telegram's servers)
- Category information

We do NOT store:
- Receipt photo files (only Telegram file IDs)
- Passwords or authentication tokens
- Payment information

### Data Retention
- **Active users**: Data retained indefinitely while you use the bot
- **Inactive users**: No automatic deletion currently implemented
- **Receipts**: Telegram file IDs retained as long as expense records exist

## Data Access

### Who Can Access Your Data
1. **You**: Full access to your own expense records via bot commands
2. **Bot administrators**: Can access database for maintenance/support
3. **Telegram**: Can access messages and photos per their policies
4. **Google Gemini**: Receives receipt photos for OCR processing

### Data Not Shared
- We do NOT sell your data to third parties
- We do NOT share individual user data with other users
- We do NOT use your data for advertising

## Your Rights

### Data Access
- Use `/list` to view your expenses
- Use `/today` or `/week` to see recent activity
- Contact bot administrator for full data export

### Data Deletion
To delete your data:
1. Delete individual expenses using `/delete <id>` command
2. Contact bot administrator for complete account deletion
3. Note: Deleting from our database does NOT delete photos from Telegram

### Data Portability
Contact bot administrator to request:
- Export of all your expense records (CSV/JSON format)
- List of all receipt photo file IDs

## Security Measures

### In Transit
- All communication encrypted via Telegram's MTProto protocol
- HTTPS used for Gemini API calls

### At Rest
- Database access restricted to authenticated applications only
- No encryption of expense data in database (considered non-sensitive)
- Receipt file IDs stored in plaintext
- Server access controlled via SSH keys and firewall rules

### Not Implemented
- ❌ Photo encryption in database (photos not stored locally)
- ❌ End-to-end encryption (relies on Telegram's security)
- ❌ Automatic data deletion after inactivity
- ❌ Two-factor authentication (uses Telegram's auth)

## Privacy Best Practices

### For Users
1. **Obscure sensitive info**: Black out credit card numbers, personal addresses before sending receipts
2. **Review before sending**: Ensure receipts don't contain passwords, account numbers, etc.
3. **Delete regularly**: Use `/delete` to remove old expenses you no longer need
4. **Understand AI processing**: Your receipts are viewed by Google's AI

### For Administrators
1. **Database access**: Limit to authorized personnel only
2. **Log review**: Monitor for unauthorized access attempts
3. **Regular backups**: Ensure data can be recovered if needed
4. **Secure credentials**: Use environment variables, never commit secrets to git

## Changes to This Policy

This privacy policy may be updated from time to time. Changes will be:
- Documented in git commit history
- Communicated to active users via bot announcement (for major changes)
- Effective immediately upon commit to master branch

## Contact

For privacy-related questions or data requests:
- Open an issue on the GitHub repository
- Contact the bot administrator via Telegram

Last updated: 2026-01-29
