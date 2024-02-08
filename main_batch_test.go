package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func buildBigBatch(n int, sameBlock bool) []*decode.EVMRPCRequestEnvelope {
	batch := make([]*decode.EVMRPCRequestEnvelope, 0, n)
	// create n requests
	for i := 0; i < n; i++ {
		block := "0x1"
		if !sameBlock {
			block = fmt.Sprintf("0x%s", strconv.FormatInt(int64(i)+1, 16))
		}
		batch = append(batch, &decode.EVMRPCRequestEnvelope{
			JSONRPCVersion: "2.0",
			ID:             i,
			Method:         "eth_getBlockByNumber",
			Params:         []interface{}{block, false},
		})
	}
	return batch
}

func TestE2ETest_ValidBatchEvmRequests(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	db, err := database.NewPostgresClient(databaseConfig)
	require.NoError(t, err)
	cleanMetricsDb(t, db)

	// NOTE! ordering matters for these tests! earlier request responses may end up in the cache.
	testCases := []struct {
		name                string
		req                 []*decode.EVMRPCRequestEnvelope
		expectedCacheHeader string
		expectedErrStatus   int
		expectedNumMetrics  int
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
			expectedNumMetrics:  1,
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
			expectedNumMetrics:  2,
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
			expectedNumMetrics:  2,
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
					ID:             nil,
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x1", false},
				},
				{
					JSONRPCVersion: "2.0",
					ID:             123456,
					Method:         "eth_getBlockByNumber",
					Params:         []interface{}{"0x3", false},
				},
			},
			expectedCacheHeader: cachemdw.CacheHitHeaderValue,
			expectedNumMetrics:  3,
		},
		{
			name:                "empty request",
			req:                 []*decode.EVMRPCRequestEnvelope{nil}, // <-- empty!
			expectedCacheHeader: cachemdw.CacheMissHeaderValue,
			expectedNumMetrics:  0,
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
			expectedNumMetrics:  1,
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
			expectedNumMetrics:  1,
		},
		{
			name:                "big-as-can-be batch, some cache hits",
			req:                 buildBigBatch(proxyServiceMaxBatchSize, false),
			expectedCacheHeader: cachemdw.CachePartialHeaderValue,
			expectedNumMetrics:  proxyServiceMaxBatchSize,
		},
		{
			name:                "big-as-can-be batch, all cache hits",
			req:                 buildBigBatch(proxyServiceMaxBatchSize, true),
			expectedCacheHeader: cachemdw.CacheHitHeaderValue,
			expectedNumMetrics:  proxyServiceMaxBatchSize,
		},
		{
			name:                "too-big batch => responds 413",
			req:                 buildBigBatch(proxyServiceMaxBatchSize+1, false),
			expectedCacheHeader: cachemdw.CacheHitHeaderValue,
			expectedErrStatus:   http.StatusRequestEntityTooLarge,
			expectedNumMetrics:  0,
		},
	}

	for _, tc := range testCases {
		startTime := time.Now()
		t.Run(tc.name, func(t *testing.T) {
			reqInJSON, err := json.Marshal(tc.req)
			require.NoError(t, err)

			resp, err := http.Post(proxyServiceURL, "application/json", bytes.NewBuffer(reqInJSON))
			require.NoError(t, err)

			if tc.expectedErrStatus != 0 {
				require.Equal(t, tc.expectedErrStatus, resp.StatusCode, "unexpected response status")
				return
			}
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

			// wait for all metrics to be created.
			// besides verification, waiting for the metrics ensures future tests don't fail b/c metrics are being processed
			waitForMetricsInWindow(t, tc.expectedNumMetrics, db, startTime, []string{})
		})
	}

	// clear all metrics & cache state to make future metrics tests less finicky
	// (more data increases the read/write to db & redis, and these tests make many db & cache entries)
	cleanMetricsDb(t, db)
	cleanUpRedis(t, redisClient)
}

func TestE2ETest_BatchEvmRequestErrorHandling(t *testing.T) {
	t.Run("no backend configured (bad gateway error)", func(t *testing.T) {
		validReq := []*decode.EVMRPCRequestEnvelope{
			newJsonRpcRequest(123, "eth_getBlockByNumber", []interface{}{"0x1", false}),
			newJsonRpcRequest("another-req", "eth_getBlockByNumber", []interface{}{"0x2", false}),
		}
		reqInJSON, err := json.Marshal(validReq)
		require.NoError(t, err)
		resp, err := http.Post(proxyUnconfiguredUrl, "application/json", bytes.NewBuffer(reqInJSON))

		require.NoError(t, err)
		require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	})

	t.Run("empty batch", func(t *testing.T) {
		emptyReq := []*decode.EVMRPCRequestEnvelope{}
		reqInJSON, err := json.Marshal(emptyReq)
		require.NoError(t, err)

		resp, err := http.Post(proxyServiceURL, "application/json", bytes.NewBuffer(reqInJSON))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "failed to read response body")

		var decoded *jsonRpcResponse // <--- NOT an array response
		err = json.Unmarshal(body, &decoded)
		require.NoError(t, err, "failed to unmarshal response into array of responses")

		require.Equal(t, -32600, decoded.JsonRpcError.Code)
		require.Equal(t, "empty batch", decoded.JsonRpcError.Message)
	})

	t.Run("unsupported method", func(t *testing.T) {
		resp, err := http.Get(proxyServiceURL) // <--- GET, not POST
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := []byte(`[{id:"almost valid json (missing double quotes around id!)"}]`)
		resp, err := http.Post(proxyServiceURL, "application/json", bytes.NewBuffer(req))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "failed to read response body")

		var decoded *jsonRpcResponse // <--- NOT an array response
		err = json.Unmarshal(body, &decoded)
		require.NoError(t, err, "failed to unmarshal response into array of responses")

		require.Equal(t, -32700, decoded.JsonRpcError.Code)
		require.Equal(t, "parse error", decoded.JsonRpcError.Message)
	})
}
