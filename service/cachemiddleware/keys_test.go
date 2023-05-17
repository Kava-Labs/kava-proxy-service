package cachemiddleware_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestGetCacheKey(t *testing.T) {
	tests := []struct {
		name              string
		giveChainID       string
		giveDecodedReq    *decode.EVMRPCRequestEnvelope
		wantKeyStartsWith string
		wantShouldErr     bool
		wantErr           error
	}{
		{
			name:        "basic",
			giveChainID: "7777",
			giveDecodedReq: &decode.EVMRPCRequestEnvelope{
				JSONRPCVersion: "2.0",
				ID:             1,
				Method:         "eth_getBlockByHash",
				Params:         []interface{}{"0x1234", true},
			},
			wantKeyStartsWith: "query:7777:0x",
			wantShouldErr:     false,
		},
		{
			name:           "nil decoded request",
			giveChainID:    "7654",
			giveDecodedReq: nil,
			wantShouldErr:  true,
			wantErr:        errors.New("decoded request is nil"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := cachemiddleware.GetQueryKey(tc.giveChainID, tc.giveDecodedReq)

			if tc.wantShouldErr {
				require.Error(t, err)
				require.Equal(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, key, "cache key should not be empty")
				require.Truef(
					t,
					strings.HasPrefix(key, tc.wantKeyStartsWith),
					"cache key should start with %s, but got %s",
					tc.wantKeyStartsWith,
					key,
				)
			}
		})
	}
}

func TestUnitTestGetChainKey(t *testing.T) {
	tests := []struct {
		name        string
		giveRequest *http.Request
		wantKey     string
	}{
		{
			name:        "port included",
			giveRequest: mustNewRequest("GET", "http://localhost:7777/test/path"),
			wantKey:     "chain:localhost:7777",
		},
		{
			name:        "api",
			giveRequest: mustNewRequest("GET", "https://api.kava.io/test/path"),
			wantKey:     "chain:api.kava.io",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := cachemiddleware.GetChainKey(tc.giveRequest.Host)

			require.Equal(t, tc.wantKey, key)
		})
	}
}
