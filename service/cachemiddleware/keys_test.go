package cachemiddleware_test

import (
	"strings"
	"testing"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestGetCacheKey(t *testing.T) {
	tests := []struct {
		name              string
		requestHost       string
		chainID           string
		decodedReq        *decode.EVMRPCRequestEnvelope
		wantKeyStartsWith string
		wantShouldErr     bool
		wantErr           error
	}{
		{
			name:        "basic",
			requestHost: "localhost:7777",
			chainID:     "7777",
			decodedReq: &decode.EVMRPCRequestEnvelope{
				JSONRPCVersion: "2.0",
				ID:             1,
				Method:         "eth_getBlockByHash",
				Params:         []interface{}{"0x1234", true},
			},
			wantKeyStartsWith: "query:7777:0x",
			wantShouldErr:     false,
		},
		{
			name:          "nil decoded request",
			requestHost:   "localhost:7778",
			decodedReq:    nil,
			wantShouldErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := cachemiddleware.GetQueryKey(tc.chainID, tc.decodedReq)
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
		name     string
		giveHost string
		wantKey  string
	}{
		{
			name:     "port included",
			giveHost: "localhost:7777",
			wantKey:  "chain:localhost:7777",
		},
		{
			name:     "api",
			giveHost: "api.kava.io",
			wantKey:  "chain:api.kava.io",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := cachemiddleware.GetChainKey(tc.giveHost)

			require.Equal(t, tc.wantKey, key)
		})
	}
}
