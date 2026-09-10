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

const (
	onlySuperadminsMsg        = "⛔ Only superadmins can use this command."
	superadminRevokeDeniedMsg = "Superadmins cannot be revoked via bot commands."
	approveUserFailedMsg      = "Failed to approve user. Please try again."
	revokeUserFailedMsg       = "Failed to revoke user. Please try again."
	failedApproveUserLogMsg   = "Failed to approve user"
	failedRevokeUserLogMsg    = "Failed to revoke user"
	targetUsernameField       = "target_username"
	targetIDField             = "target_id"
	superadminIDLineFmt       = "  ID: <code>%d</code>\n"
	superadminUsernameLineFmt = "  @%s\n"
	approveUsageMsg           = "Usage: <code>/approve &lt;user_id&gt;</code> or <code>/approve @username</code>"
	revokeUsageMsg            = "Usage: <code>/revoke &lt;user_id&gt;</code> or <code>/revoke @username</code>"
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

// requireSuperadmin checks whether the given user is a superadmin. When they
// are not, it replies with an access-denied message and returns false so the
// caller can stop processing.
func (b *Bot) requireSuperadmin(ctx context.Context, tg TelegramAPI, chatID, userID int64, username string) bool {
	if b.cfg.IsSuperAdmin(userID, username) {
		return true
	}

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   onlySuperadminsMsg,
	})
	return false
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

	if !b.requireSuperadmin(ctx, tg, chatID, userID, username) {
		return
	}

	args := extractAdminArgs(update.Message.Text)
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      approveUsageMsg,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Try parsing as user ID first.
	if targetID, err := strconv.ParseInt(args, 10, 64); err == nil {
		// Telegram user IDs are positive. user_id = 0 is the sentinel for
		// "approved by @username only" rows, so /approve 0 reaches the
		// INSERT with targetID = 0 and username = "" and mints a (0, "")
		// orphan row that sits outside both partial unique indexes (so it
		// never deduplicates, once per call) and that IsApproved never
		// matches (so it grants no access); it also cannot be removed via
		// /revoke, whose DELETE excludes user_id = 0. ParseInt collapses
		// forms like "+0" and "-0" to 0, and negative IDs are never real
		// targets either. Reject the whole non-positive range as invalid
		// usage, mirroring /revoke, which also avoids a misleading
		// "User <code>0</code> has been approved." reply.
		if targetID <= 0 {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      approveUsageMsg,
				ParseMode: models.ParseModeHTML,
			})
			return
		}
		if err := b.approvedUserRepo.Approve(ctx, targetID, "", userID); err != nil {
			logger.Log.Error().Err(err).Int64(targetIDField, targetID).Msg(failedApproveUserLogMsg)
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   approveUserFailedMsg,
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
	// username = "" is the sentinel for "approved by ID only" rows and is never
	// a real target (e.g. "/approve @" trims to ""). Reject it as invalid usage
	// so a single /approve @ cannot reach the INSERT with username = "" (which
	// mints a (0, "") orphan row outside both partial unique indexes that never
	// deduplicates, grants no access, and cannot be removed via /revoke @),
	// mirroring /revoke, which also avoids a misleading "User <code>@</code>
	// has been approved." reply.
	if targetUsername == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      approveUsageMsg,
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	if err := b.approvedUserRepo.ApproveByUsername(ctx, targetUsername, userID); err != nil {
		logger.Log.Error().Err(err).Str(targetUsernameField, targetUsername).Msg(failedApproveUserLogMsg)
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   approveUserFailedMsg,
		})
		return
	}
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("User <code>@%s</code> has been approved.", escapeHTML(targetUsername)),
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

	if !b.requireSuperadmin(ctx, tg, chatID, userID, username) {
		return
	}

	args := extractAdminArgs(update.Message.Text)
	if args == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      revokeUsageMsg,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Try parsing as user ID first.
	if targetID, err := strconv.ParseInt(args, 10, 64); err == nil {
		// Telegram user IDs are positive. user_id = 0 is the sentinel for
		// "approved by @username only" rows, so /revoke 0 would reach an
		// unguarded DELETE ... WHERE user_id = 0 and wipe every
		// username-only approval; ParseInt also collapses forms like "+0"
		// and "-0" to 0. Negative IDs are never real targets either.
		// Reject the whole non-positive range as invalid usage, which also
		// avoids a misleading "User <code>0</code> has been revoked." reply.
		if targetID <= 0 {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    chatID,
				Text:      revokeUsageMsg,
				ParseMode: models.ParseModeHTML,
			})
			return
		}
		if b.cfg.IsSuperAdmin(targetID, "") {
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   superadminRevokeDeniedMsg,
			})
			return
		}
		if err := b.approvedUserRepo.Revoke(ctx, targetID); err != nil {
			logger.Log.Error().Err(err).Int64(targetIDField, targetID).Msg(failedRevokeUserLogMsg)
			_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   revokeUserFailedMsg,
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
	// username = '' is the sentinel for "approved by ID only" rows and is never
	// a real target (e.g. "/revoke @" trims to ""). Reject it as invalid usage so
	// a single /revoke @ cannot reach an unguarded DELETE ... WHERE
	// LOWER(username) = '' that would wipe every by-ID approval, and cannot
	// produce a misleading "User <code>@</code> has been revoked." reply.
	if targetUsername == "" {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      revokeUsageMsg,
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	if b.cfg.IsSuperAdmin(0, targetUsername) {
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   superadminRevokeDeniedMsg,
		})
		return
	}
	if err := b.approvedUserRepo.RevokeByUsername(ctx, targetUsername); err != nil {
		logger.Log.Error().Err(err).Str(targetUsernameField, targetUsername).Msg(failedRevokeUserLogMsg)
		_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   revokeUserFailedMsg,
		})
		return
	}
	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("User <code>@%s</code> has been revoked.", escapeHTML(targetUsername)),
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

	if !b.requireSuperadmin(ctx, tg, chatID, userID, username) {
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>Superadmins:</b>\n")
	for _, id := range b.cfg.WhitelistedUserIDs {
		fmt.Fprintf(&sb, superadminIDLineFmt, id)
	}
	for _, u := range b.cfg.WhitelistedUsernames {
		fmt.Fprintf(&sb, superadminUsernameLineFmt, escapeHTML(u))
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
		for i := range approved {
			u := approved[i]
			switch {
			case u.UserID != 0 && u.Username != "":
				fmt.Fprintf(&sb, "  ID: <code>%d</code> (@%s)\n", u.UserID, escapeHTML(u.Username))
			case u.UserID != 0:
				fmt.Fprintf(&sb, superadminIDLineFmt, u.UserID)
			default:
				fmt.Fprintf(&sb, superadminUsernameLineFmt, escapeHTML(u.Username))
			}
		}
	}

	_, _ = tg.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
}
