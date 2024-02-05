package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestE2ETest_ValidBatchEvmRequests(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	// NOTE! ordering matters for these tests! earlier request responses may end up in the cache.
	testCases := []struct {
		name                string
		req                 []*decode.EVMRPCRequestEnvelope
		expectedCacheHeader string
	}{
		{
			name: "first request, valid & not coming from the cache",
			req: []*decode.EVMRPCRequestEnvelope{
				{
					JSONRPCVersion: "2.0",
					ID:             "magic!",
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x1", false},
				},
			},
			expectedCacheHeader: cachemdw.CacheMissHeaderValue,
		},
		{
			name: "multiple requests, valid & none coming from the cache",
			req: []*decode.EVMRPCRequestEnvelope{
				{
					JSONRPCVersion: "2.0",
					ID:             "magic!",
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x2", false},
				},
				{
					JSONRPCVersion: "2.0",
					ID:             123456,
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x3", false},
				},
			},
			expectedCacheHeader: cachemdw.CacheMissHeaderValue,
		},
		{
			name: "multiple requests, valid & some coming from the cache",
			req: []*decode.EVMRPCRequestEnvelope{
				{
					JSONRPCVersion: "2.0",
					ID:             "magic!",
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x2", false},
				},
				{
					JSONRPCVersion: "2.0",
					ID:             123456,
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x4", false},
				},
			},
			expectedCacheHeader: cachemdw.CachePartialHeaderValue,
		},
		{
			name: "multiple requests, valid & all coming from the cache",
			req: []*decode.EVMRPCRequestEnvelope{
				{
					JSONRPCVersion: "2.0",
					ID:             "magic!",
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x2", false},
				},
				{
					JSONRPCVersion: "2.0",
					ID:             123456,
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x4", false},
				},
			},
			expectedCacheHeader: cachemdw.CacheHitHeaderValue,
		},
		{
			name:                "empty request",
			req:                 []*decode.EVMRPCRequestEnvelope{nil}, // <-- empty!
			expectedCacheHeader: cachemdw.CacheMissHeaderValue,
		},
		{
			name: "empty & non-empty requests, partial cache hit",
			req: []*decode.EVMRPCRequestEnvelope{
				nil, // <-- empty!
				{
					JSONRPCVersion: "2.0",
					ID:             "this block is in the cache",
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x1", false},
				},
			},
			expectedCacheHeader: cachemdw.CachePartialHeaderValue,
		},
		{
			name: "empty & non-empty requests, cache miss",
			req: []*decode.EVMRPCRequestEnvelope{
				nil, // <-- empty!
				{
					JSONRPCVersion: "2.0",
					ID:             "this block is NOT in the cache",
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0xa", false},
				},
			},
			expectedCacheHeader: cachemdw.CacheMissHeaderValue,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqInJSON, err := json.Marshal(tc.req)
			require.NoError(t, err)

			resp, err := http.Post(proxyServiceURL, "application/json", bytes.NewBuffer(reqInJSON))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "failed to read response body")

			var decoded []*jsonRpcResponse
			err = json.Unmarshal(body, &decoded)
			require.NoError(t, err, "failed to unmarshal response into array of responses")

			// expect same number of responses as requests
			require.Len(t, decoded, len(tc.req))

			// expect matching ids
			for i, d := range decoded {
				var reqId interface{} = nil
				if tc.req[i] != nil {
					reqId = tc.req[i].ID
				}
				// EqualValues here because json ints unmarshal as float64s
				require.EqualValues(t, reqId, d.Id)
			}

			// check expected cache status header
			require.Equal(t, tc.expectedCacheHeader, resp.Header.Get(cachemdw.CacheHeaderKey))

			// verify CORS header
			require.Equal(t, resp.Header[accessControlAllowOriginHeaderName], []string{"*"})
		})
	}
}

// Errors to test:
// - no backend configures (500 error)
// - empty batch
// - unsupported method
