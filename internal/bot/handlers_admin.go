package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
)

// extractAdminArgs extracts command arguments while preserving @username args.
// Unlike extractCommandArgs, it only strips the command word (and any bot mention
// attached to it), preserving @username as an argument rather than stripping it.
func extractAdminArgs(text string) string {
	// Split on first space to separate command from args.
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// handleApprove handles the /approve command to approve a user.
func (b *Bot) handleApprove(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleApproveCore(ctx, tgBot, update)
}

// handleApproveCore is the testable implementation of handleApprove.
func (b *Bot) handleApproveCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	username := update.Message.From.Username

	if !b.cfg.IsSuperAdmin(userID, username) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "⛔ Only superadmins can use this command.",
		})
		return
	}

	args := extractAdminArgs(update.Message.Text)
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "Usage: <code>/approve &lt;user_id&gt;</code> or <code>/approve @username</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Try parsing as user ID first.
	if targetID, err := strconv.ParseInt(args, 10, 64); err == nil {
		if err := b.approvedUserRepo.Approve(ctx, targetID, "", userID); err != nil {
			logger.Log.Error().Err(err).Int64("target_id", targetID).Msg("Failed to approve user")
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Failed to approve user. Please try again.",
			})
			return
		}
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("User <code>%d</code> has been approved.", targetID),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Treat as username.
	targetUsername := strings.TrimPrefix(args, "@")
	if err := b.approvedUserRepo.ApproveByUsername(ctx, targetUsername, userID); err != nil {
		logger.Log.Error().Err(err).Str("target_username", targetUsername).Msg("Failed to approve user")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to approve user. Please try again.",
		})
		return
	}
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("User <code>@%s</code> has been approved.", targetUsername),
		ParseMode: models.ParseModeHTML,
	})
}

// handleRevoke handles the /revoke command to revoke a user.
func (b *Bot) handleRevoke(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleRevokeCore(ctx, tgBot, update)
}

// handleRevokeCore is the testable implementation of handleRevoke.
func (b *Bot) handleRevokeCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	username := update.Message.From.Username

	if !b.cfg.IsSuperAdmin(userID, username) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "⛔ Only superadmins can use this command.",
		})
		return
	}

	args := extractAdminArgs(update.Message.Text)
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "Usage: <code>/revoke &lt;user_id&gt;</code> or <code>/revoke @username</code>",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Try parsing as user ID first.
	if targetID, err := strconv.ParseInt(args, 10, 64); err == nil {
		if b.cfg.IsSuperAdmin(targetID, "") {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Superadmins cannot be revoked via bot commands.",
			})
			return
		}
		if err := b.approvedUserRepo.Revoke(ctx, targetID); err != nil {
			logger.Log.Error().Err(err).Int64("target_id", targetID).Msg("Failed to revoke user")
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Failed to revoke user. Please try again.",
			})
			return
		}
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("User <code>%d</code> has been revoked.", targetID),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Treat as username.
	targetUsername := strings.TrimPrefix(args, "@")
	if b.cfg.IsSuperAdmin(0, targetUsername) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Superadmins cannot be revoked via bot commands.",
		})
		return
	}
	if err := b.approvedUserRepo.RevokeByUsername(ctx, targetUsername); err != nil {
		logger.Log.Error().Err(err).Str("target_username", targetUsername).Msg("Failed to revoke user")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to revoke user. Please try again.",
		})
		return
	}
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("User <code>@%s</code> has been revoked.", targetUsername),
		ParseMode: models.ParseModeHTML,
	})
}

// handleUsers handles the /users command to list authorized users.
func (b *Bot) handleUsers(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.handleUsersCore(ctx, tgBot, update)
}

// handleUsersCore is the testable implementation of handleUsers.
func (b *Bot) handleUsersCore(ctx context.Context, tg TelegramAPI, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	username := update.Message.From.Username

	if !b.cfg.IsSuperAdmin(userID, username) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "⛔ Only superadmins can use this command.",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>Superadmins:</b>\n")
	for _, id := range b.cfg.WhitelistedUserIDs {
		sb.WriteString(fmt.Sprintf("  ID: <code>%d</code>\n", id))
	}
	for _, u := range b.cfg.WhitelistedUsernames {
		sb.WriteString(fmt.Sprintf("  @%s\n", u))
	}

	approved, err := b.approvedUserRepo.GetAll(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to get approved users")
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to fetch approved users.",
		})
		return
	}

	sb.WriteString("\n<b>Approved Users:</b>\n")
	if len(approved) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, u := range approved {
			switch {
			case u.UserID != 0 && u.Username != "":
				sb.WriteString(fmt.Sprintf("  ID: <code>%d</code> (@%s)\n", u.UserID, u.Username))
			case u.UserID != 0:
				sb.WriteString(fmt.Sprintf("  ID: <code>%d</code>\n", u.UserID))
			default:
				sb.WriteString(fmt.Sprintf("  @%s\n", u.Username))
			}
		}
	}

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
}
