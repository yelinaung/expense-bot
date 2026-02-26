package telemetry

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestInstrumentedTransport(t *testing.T) {
	t.Parallel()

	t.Run("uses default transport when base is nil", func(t *testing.T) {
		t.Parallel()
		transport := InstrumentedTransport(nil)
		require.NotNil(t, transport)
	})

	t.Run("wraps custom transport and executes request", func(t *testing.T) {
		t.Parallel()
		called := false
		base := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			called = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(io.Reader(http.NoBody)),
				Header:     make(http.Header),
			}, nil
		})

		transport := InstrumentedTransport(base)
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"http://example.com",
			nil,
		)
		require.NoError(t, err)
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		require.True(t, called)
	})
}
