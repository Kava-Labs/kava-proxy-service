package cache_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kava-labs/kava-proxy-service/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/stretchr/testify/require"
)

func TestUnitTestGetCacheKey(t *testing.T) {
	tests := []struct {
		name              string
		r                 *http.Request
		chainID           string
		decodedReq        *decode.EVMRPCRequestEnvelope
		wantKeyStartsWith string
		wantShouldErr     bool
		wantErr           error
	}{
		{
			name: "basic",
			r: &http.Request{
				URL: &url.URL{
					Path: "/",
				},
			},
			chainID: "7777",
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
			name: "nil decoded request",
			r: &http.Request{
				URL: &url.URL{
					Path: "/",
				},
			},
			decodedReq:    nil,
			wantShouldErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := cache.GetQueryKey(tc.r, tc.chainID, tc.decodedReq)
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
