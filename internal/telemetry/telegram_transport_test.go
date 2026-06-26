package telemetry

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTelegramMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"send message", "/bot12345:secret/sendMessage", "sendMessage"},
		{"get updates", "/bot12345:secret/getUpdates", "getUpdates"},
		{"get file", "/bot12345:secret/getFile", "getFile"},
		{"empty trailing", "/bot12345:secret/", "unknown"},
		{"no slash", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, telegramMethod(tt.path))
		})
	}
}

func okResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(http.NoBody),
		Header:     make(http.Header),
	}
}

// roundTripForMethod sends a request for the given Telegram method through the
// transport and returns the method the base transport actually saw.
func roundTripForMethod(t *testing.T, method string) string {
	t.Helper()
	var gotMethod string
	base := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotMethod = telegramMethod(r.URL.Path)
		return okResponse(), nil
	})
	tr := telegramTransport{base: base}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://api.telegram.org/bot12345:secret/"+method, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return gotMethod
}

func TestTelegramTransport_SkipsGetUpdates(t *testing.T) {
	t.Parallel()
	require.Equal(t, "getUpdates", roundTripForMethod(t, "getUpdates"))
}

func TestTelegramTransport_TracesSendMessage(t *testing.T) {
	t.Parallel()
	require.Equal(t, "sendMessage", roundTripForMethod(t, "sendMessage"))
}

func TestTelegramHTTPClient(t *testing.T) {
	t.Parallel()

	c := TelegramHTTPClient(time.Minute)
	require.NotNil(t, c)
	require.Equal(t, time.Minute, c.Timeout)
	require.IsType(t, telegramTransport{}, c.Transport)
}
