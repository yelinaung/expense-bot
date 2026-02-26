package bot

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

type fakeHTTPClient struct {
	calls int
}

func (c *fakeHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	c.calls++
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(`{"ok":false}`)),
		Header:     make(http.Header),
	}, nil
}

func TestDefaultHandler_UnknownText_NoPanic(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)

	client := &fakeHTTPClient{}
	tgBot, err := tgbot.New(
		"123:TESTTOKEN",
		tgbot.WithSkipGetMe(),
		tgbot.WithHTTPClient(time.Second, client),
		tgbot.WithServerURL("http://example.com"),
	)
	require.NoError(t, err)

	update := &tgmodels.Update{
		Message: &tgmodels.Message{
			Chat: tgmodels.Chat{ID: 12345},
			From: &tgmodels.User{ID: 3001},
			Text: "this is not an expense input",
		},
	}

	require.NotPanics(t, func() {
		b.defaultHandler(context.Background(), tgBot, update)
	})
	require.Positive(t, client.calls)
}
