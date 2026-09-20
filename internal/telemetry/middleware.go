package telemetry

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gitlab.com/yelinaung/expense-bot/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	telegramChatId = "telegram.chat_id"
	telegramUserId = "telegram.user_id"

	// commandTokenMaxLength bounds the length of a sanitized command token
	// (excluding the leading "/") that may appear in a span or metric name.
	// The OTel Trace SDK applies no cap on span-name length, so length is
	// bounded here to keep a single message from producing an oversized name.
	commandTokenMaxLength = 32
)

var tracer = otel.Tracer("expense-bot/telegram")

// allowedCommandChars matches the body of a sanitized command token (the part
// after the leading "/"). Restricting to lowercase ASCII letters, digits, and
// underscore keeps span names free of unicode, whitespace, and punctuation.
var allowedCommandChars = regexp.MustCompile(`^[a-z0-9_]+$`)

// TracingMiddleware returns a bot middleware that creates a root span per
// Telegram update and records handler duration / count metrics. knownCommands
// is the set of registered command tokens (e.g. "/add") eligible to appear as
// the span-name suffix; any command not in the set collapses to "unknown" so
// that attacker-supplied text cannot inflate trace span or metric-label
// cardinality.
func TracingMiddleware(metrics *BotMetrics, knownCommands ...string) func(next bot.HandlerFunc) bot.HandlerFunc {
	allowed := make(map[string]struct{}, len(knownCommands))
	for _, c := range knownCommands {
		allowed[c] = struct{}{}
	}
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			spanName := classifyUpdate(update, allowed)
			attrs := updateAttributes(update)

			ctx, span := tracer.Start(
				ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attrs...),
			)
			defer span.End()

			start := time.Now()
			if metrics != nil {
				metrics.HandlersInFlight.Add(ctx, 1)
				defer metrics.HandlersInFlight.Add(ctx, -1)
			}

			defer func() {
				if r := recover(); r != nil {
					span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", r))
					span.RecordError(fmt.Errorf("panic: %v", r))
					if metrics != nil {
						recordHandlerMetrics(ctx, metrics, spanName, "panic", start)
					}
					panic(r)
				}
			}()

			next(ctx, b, update)

			if metrics != nil {
				recordHandlerMetrics(ctx, metrics, spanName, "ok", start)
			}
		}
	}
}

func recordHandlerMetrics(ctx context.Context, m *BotMetrics, handler, status string, start time.Time) {
	attrs := otelmetric.WithAttributes(
		attribute.String("handler", handler),
		attribute.String("status", status),
	)
	m.HandlerCount.Add(ctx, 1, attrs)
	m.HandlerDuration.Record(ctx, time.Since(start).Seconds(), attrs)
}

func classifyUpdate(update *models.Update, knownCommands map[string]struct{}) string {
	if update.Message != nil {
		if update.Message.Voice != nil {
			return "telegram.voice"
		}
		if len(update.Message.Photo) > 0 {
			return "telegram.photo"
		}
		if update.Message.Text != "" {
			cmd := extractCommand(update.Message.Text)
			if cmd != "" {
				return "telegram.command " + resolveCommand(cmd, knownCommands)
			}
			return "telegram.text"
		}
		if update.Message.Document != nil {
			return "telegram.document"
		}
		return "telegram.message"
	}
	if update.CallbackQuery != nil {
		prefix := extractCallbackPrefix(update.CallbackQuery.Data)
		return "telegram.callback " + prefix
	}
	if update.EditedMessage != nil {
		return "telegram.edited_message"
	}
	return "telegram.update"
}

// extractCommand returns the leading command token of text, normalized: the
// optional "@botname" suffix is stripped, the token is lowercased, and its
// length is bounded. The returned token retains its leading "/" as Telegram
// commands do. It returns "" when text does not begin with "/", so callers
// can distinguish "not a command" from a (possibly unrecognized) command.
func extractCommand(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	cmd, _, _ := strings.Cut(text, " ")
	// Strip the optional "@botname" suffix Telegram appends in group chats.
	if idx := strings.Index(cmd, "@"); idx > 0 {
		cmd = cmd[:idx]
	}
	cmd = strings.ToLower(cmd)
	// Bound length so one message cannot create an oversized span name; the
	// Trace SDK applies no span-name length cap.
	if len(cmd) > commandTokenMaxLength+1 {
		cmd = cmd[:commandTokenMaxLength+1]
	}
	return cmd
}

// resolveCommand maps a sanitized command token to the low-cardinality suffix
// used in span and metric names. Tokens with disallowed characters, a bare
// "/", or that are not in the known set collapse to "unknown" so the set of
// span names is bounded to the registered commands plus a single "unknown".
func resolveCommand(cmd string, knownCommands map[string]struct{}) string {
	if len(cmd) < 2 || !allowedCommandChars.MatchString(cmd[1:]) {
		return "unknown"
	}
	if _, ok := knownCommands[cmd]; !ok {
		return "unknown"
	}
	return cmd
}

func extractCallbackPrefix(data string) string {
	// Return up to second underscore for readability:
	// "receipt_confirm_123" -> "receipt_confirm"
	parts := strings.SplitN(data, "_", 3)
	if len(parts) >= 2 {
		return parts[0] + "_" + parts[1]
	}
	return data
}

func updateAttributes(update *models.Update) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "telegram"),
	}

	switch {
	case update.Message != nil:
		attrs = append(
			attrs,
			attribute.String(telegramChatId, logger.HashChatID(update.Message.Chat.ID)),
		)
		if update.Message.From != nil {
			attrs = append(
				attrs,
				attribute.String(telegramUserId, logger.HashUserID(update.Message.From.ID)),
			)
		}
	case update.CallbackQuery != nil:
		attrs = append(
			attrs,
			attribute.String(telegramUserId, logger.HashUserID(update.CallbackQuery.From.ID)),
		)
		if update.CallbackQuery.Message.Message != nil {
			attrs = append(
				attrs,
				attribute.String(telegramChatId, logger.HashChatID(update.CallbackQuery.Message.Message.Chat.ID)),
			)
		}
	case update.EditedMessage != nil:
		attrs = append(
			attrs,
			attribute.String(telegramChatId, logger.HashChatID(update.EditedMessage.Chat.ID)),
		)
		if update.EditedMessage.From != nil {
			attrs = append(
				attrs,
				attribute.String(telegramUserId, logger.HashUserID(update.EditedMessage.From.ID)),
			)
		}
	}

	return attrs
}
