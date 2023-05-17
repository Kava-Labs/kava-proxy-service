package cachemiddleware_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustNewRequest(method string, url string) *http.Request {
	request, err := http.NewRequest(
		method,
		url,
		nil,
	)
	if err != nil {
		panic(err)
	}

	return request
}

func mustMarshalJsonRawMessage(t *testing.T, v json.RawMessage) string {
	t.Helper()

	bz, err := json.Marshal(v)
	require.NoError(t, err)

	return string(bz)
}

func toJsonRawMessage(t *testing.T, v any) json.RawMessage {
	t.Helper()

	bz, err := json.Marshal(v)
	require.NoError(t, err)

	return bz
}
