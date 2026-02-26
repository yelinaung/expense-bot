package telemetry

import (
	"context"
	"fmt"
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
)

var tracer = otel.Tracer("expense-bot/telegram")

// TracingMiddleware returns a bot middleware that creates a root span per
// Telegram update and records handler duration / count metrics.
func TracingMiddleware(metrics *BotMetrics) func(next bot.HandlerFunc) bot.HandlerFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			spanName := classifyUpdate(update)
			attrs := updateAttributes(update)

			ctx, span := tracer.Start(ctx, spanName,
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

func classifyUpdate(update *models.Update) string {
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
				return "telegram.command " + cmd
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

func extractCommand(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	cmd := strings.SplitN(text, " ", 2)[0]
	// Strip @botname suffix
	if idx := strings.Index(cmd, "@"); idx > 0 {
		cmd = cmd[:idx]
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
		attrs = append(attrs,
			attribute.String(telegramChatId, logger.HashChatID(update.Message.Chat.ID)),
		)
		if update.Message.From != nil {
			attrs = append(attrs,
				attribute.String(telegramUserId, logger.HashUserID(update.Message.From.ID)),
			)
		}
	case update.CallbackQuery != nil:
		attrs = append(attrs,
			attribute.String(telegramUserId, logger.HashUserID(update.CallbackQuery.From.ID)),
		)
		if update.CallbackQuery.Message.Message != nil {
			attrs = append(attrs,
				attribute.String(telegramChatId, logger.HashChatID(update.CallbackQuery.Message.Message.Chat.ID)),
			)
		}
	case update.EditedMessage != nil:
		attrs = append(attrs,
			attribute.String(telegramChatId, logger.HashChatID(update.EditedMessage.Chat.ID)),
		)
		if update.EditedMessage.From != nil {
			attrs = append(attrs,
				attribute.String(telegramUserId, logger.HashUserID(update.EditedMessage.From.ID)),
			)
		}
	}

	return attrs
}
