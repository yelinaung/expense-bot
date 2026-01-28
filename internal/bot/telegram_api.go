package bot

import (
	tgbot "github.com/go-telegram/bot"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
)

// TelegramAPI is an alias to the interface defined in mocks package.
// The interface is defined in mocks to avoid import cycles.
type TelegramAPI = mocks.TelegramAPI

// Compile-time check that the real bot satisfies the interface.
var _ TelegramAPI = (*tgbot.Bot)(nil)
