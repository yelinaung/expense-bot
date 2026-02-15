package bot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

// maxTagsPerCommand is the maximum number of tags allowed in a single /tag command.
const maxTagsPerCommand = 10

// validTagNameRegex validates a bare tag name (without the # prefix).
var validTagNameRegex = regexp.MustCompile(`^[a-zA-Z]\w{0,29}$`)

// isValidTagName checks whether a tag name is valid.
func isValidTagName(name string) bool {
	return len(name) <= appmodels.MaxTagNameLength && validTagNameRegex.MatchString(name)
}

// escapeHTML escapes HTML special characters for safe interpolation in Telegram HTML messages.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// handleTag handles the /tag command to add tags to an expense.
func (b *Bot) handleTag(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleTagCore(ctx, tgBot, update)
}

// handleTagCore is the testable implementation of handleTag.
func (b *Bot) handleTagCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	expenseNum, tagNames, parseErrText := parseTagCommand(update.Message.Text)
	if parseErrText != "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      parseErrText,
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	if len(tagNames) > maxTagsPerCommand {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Too many tags. Maximum %d tags per command.", maxTagsPerCommand),
		})
		return
	}

	expense, err := b.expenseRepo.GetByUserAndNumber(ctx, userID, expenseNum)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Expense #%d not found.", expenseNum),
		})
		return
	}

	if expense.UserID != userID {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only tag your own expenses.",
		})
		return
	}

	tagIDs, addedNames, err := b.resolveTagIDs(ctx, tagNames)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      err.Error(),
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	if len(tagIDs) == 0 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ No valid tags provided.",
		})
		return
	}

	if err := b.tagRepo.AddTagsToExpense(ctx, expense.ID, tagIDs); err != nil {
		logger.Log.Error().Err(err).Int("expense_id", expense.ID).Msg("Failed to add tags to expense")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to add tags. Please try again.",
		})
		return
	}

	// Fetch current tags for the expense.
	currentTags, err := b.tagRepo.GetByExpenseID(ctx, expense.ID)
	if err != nil {
		logger.Log.Warn().Err(err).Int("expense_id", expense.ID).Msg("Failed to fetch tags for confirmation")
	}

	text := buildTagConfirmationText(addedNames, expenseNum, currentTags)

	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /tag response")
	}
}

func parseTagCommand(text string) (int64, []string, string) {
	args := extractCommandArgs(text, "/tag")
	if args == "" {
		return 0, nil, "❌ Usage: <code>/tag &lt;id&gt; #tag1 [#tag2] ...</code>"
	}
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return 0, nil, "❌ Usage: <code>/tag &lt;id&gt; #tag1 [#tag2] ...</code>"
	}
	expenseNum, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, nil, "❌ Invalid expense ID. Use: <code>/tag &lt;id&gt; #tag1 [#tag2] ...</code>"
	}
	return expenseNum, parts[1:], ""
}

func (b *Bot) resolveTagIDs(ctx context.Context, tagNames []string) ([]int, []string, error) {
	tagIDs := make([]int, 0, len(tagNames))
	addedNames := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		name = strings.ToLower(strings.TrimPrefix(name, "#"))
		if name == "" {
			continue
		}
		if !isValidTagName(name) {
			return nil, nil, fmt.Errorf(
				"❌ Invalid tag name '%s'. Tags must start with a letter, contain only letters/numbers/underscores, and be at most %d characters",
				name,
				appmodels.MaxTagNameLength,
			)
		}
		tag, err := b.tagRepo.GetOrCreate(ctx, name)
		if err != nil {
			logger.Log.Warn().Err(err).Str("tag", name).Msg("Failed to create tag")
			continue
		}
		tagIDs = append(tagIDs, tag.ID)
		addedNames = append(addedNames, "#"+name)
	}
	return tagIDs, addedNames, nil
}

func buildTagConfirmationText(
	addedNames []string,
	expenseNum int64,
	currentTags []appmodels.Tag,
) string {
	if len(currentTags) == 0 {
		return fmt.Sprintf("✅ Added %s to expense #%d.",
			strings.Join(addedNames, ", "),
			expenseNum)
	}

	currentNames := make([]string, 0, len(currentTags))
	for _, tag := range currentTags {
		currentNames = append(currentNames, "#"+escapeHTML(tag.Name))
	}
	return fmt.Sprintf("✅ Added %s to expense #%d.\n🏷️ Tags: %s",
		strings.Join(addedNames, ", "),
		expenseNum,
		strings.Join(currentNames, " "))
}

// handleUntag handles the /untag command to remove a tag from an expense.
func (b *Bot) handleUntag(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleUntagCore(ctx, tgBot, update)
}

// handleUntagCore is the testable implementation of handleUntag.
func (b *Bot) handleUntagCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := extractCommandArgs(update.Message.Text, "/untag")

	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/untag &lt;id&gt; #tag</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Usage: <code>/untag &lt;id&gt; #tag</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expenseNum, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ Invalid expense ID. Use: <code>/untag &lt;id&gt; #tag</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	expense, err := b.expenseRepo.GetByUserAndNumber(ctx, userID, expenseNum)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Expense #%d not found.", expenseNum),
		})
		return
	}

	if expense.UserID != userID {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ You can only untag your own expenses.",
		})
		return
	}

	tagName := strings.ToLower(strings.TrimPrefix(parts[1], "#"))
	if !isValidTagName(tagName) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Invalid tag name. Tags must start with a letter, contain only letters/numbers/underscores, and be at most %d characters.", appmodels.MaxTagNameLength),
		})
		return
	}

	tag, err := b.tagRepo.GetByName(ctx, tagName)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Tag '%s' not found.", tagName),
		})
		return
	}

	if err := b.tagRepo.RemoveTagFromExpense(ctx, expense.ID, tag.ID); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to remove tag from expense")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to remove tag. Please try again.",
		})
		return
	}

	text := fmt.Sprintf("✅ Removed #%s from expense #%d.", tagName, expenseNum)

	_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to send /untag response")
	}
}

// handleTags handles the /tags command to list all tags or filter expenses by tag.
func (b *Bot) handleTags(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleTagsCore(ctx, tgBot, update)
}

// handleTagsCore is the testable implementation of handleTags.
func (b *Bot) handleTagsCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	args := extractCommandArgs(update.Message.Text, "/tags")

	if args == "" {
		// List all tags.
		tags, err := b.tagRepo.GetAllByUserID(ctx, userID)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to fetch tags")
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Failed to fetch tags. Please try again.",
			})
			return
		}

		if len(tags) == 0 {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      "🏷️ No tags found.\n\nAdd tags inline: <code>5.50 Coffee #work</code>\nOr use: <code>/tag &lt;id&gt; &lt;tag&gt;</code>",
				ParseMode: models.ParseModeHTML,
			})
			return
		}

		var sb strings.Builder
		sb.WriteString("🏷️ <b>Tags</b>\n\n")
		for i, tag := range tags {
			sb.WriteString(fmt.Sprintf("%d. #%s\n", i+1, escapeHTML(tag.Name)))
		}

		_, err = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      sb.String(),
			ParseMode: models.ParseModeHTML,
		})
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to send /tags response")
		}
		return
	}

	// Filter expenses by tag name.
	tagName := strings.ToLower(strings.TrimPrefix(args, "#"))
	if !isValidTagName(tagName) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Invalid tag name. Tags must start with a letter, contain only letters/numbers/underscores, and be at most %d characters.", appmodels.MaxTagNameLength),
		})
		return
	}

	tag, err := b.tagRepo.GetByName(ctx, tagName)
	if err != nil {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Tag '%s' not found.\n\nUse /tags to see all tags.", tagName),
		})
		return
	}

	expenses, err := b.tagRepo.GetExpensesByTagID(ctx, userID, tag.ID, 20)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to fetch expenses by tag")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch expenses. Please try again.",
		})
		return
	}

	header := fmt.Sprintf("🏷️ <b>Expenses tagged #%s</b>", escapeHTML(tag.Name))
	b.sendExpenseListCore(ctx, tg, chatID, expenses, header)
}
