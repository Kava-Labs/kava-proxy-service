package main_test

import (
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"math/big"
	"strings"
	"testing"

	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

func TestNoMetricsE2ETestProxyReturnsNonZeroLatestBlockHeader(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	header, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	require.Greater(t, int(header.Number.Int64()), 0)
}

func TestNoMetricsE2ETestProxyProxiesForMultipleHosts(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	header, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	require.Greater(t, int(header.Number.Int64()), 0)

	pruningClient, err := ethclient.Dial(proxyServicePruningURL)

	require.NoError(t, err)

	header, err = pruningClient.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	require.Greater(t, int(header.Number.Int64()), 0)
}

func TestNoMetricsE2ETestProxyTracksBlockNumberForEth_getBlockByNumberRequest(t *testing.T) {
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	response, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	requestBlockNumber := response.Number

	// make request to api and track start / end time of the request to
	_, err = client.HeaderByNumber(testContext, requestBlockNumber)
	require.NoError(t, err)
}

func TestNoMetricsE2ETestProxyTracksBlockNumberForMethodsWithBlockNumberParam(t *testing.T) {
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)
	testRandomHash := common.HexToHash(testRandomAddressHex)

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	latestBlock, err := client.HeaderByNumber(testContext, nil)

	require.NoError(t, err)

	requestBlockNumber := latestBlock.Number

	// make requests to api and track start / end time of the request
	// we don't check response errors because the proxy will create metrics
	// for each request whether the kava node api returns an error or not
	// and if it doesn't the test itself will fail due to missing metrics

	// eth_getBalance
	_, err = client.BalanceAt(testContext, testAddress, requestBlockNumber)
	require.NoError(t, err)

	// eth_getStorageAt
	_, err = client.StorageAt(testContext, testAddress, testRandomHash, requestBlockNumber)
	require.NoError(t, err)

	// eth_getTransactionCount
	_, err = client.NonceAt(testContext, testAddress, requestBlockNumber)
	require.NoError(t, err)

	// eth_getBlockTransactionCountByNumber
	_, err = client.PendingTransactionCount(testContext)
	require.NoError(t, err)

	// eth_getCode
	_, err = client.CodeAt(testContext, testAddress, requestBlockNumber)
	require.NoError(t, err)

	// eth_getBlockByNumber
	_, err = client.HeaderByNumber(testContext, requestBlockNumber)
	require.NoError(t, err)

	// eth_call
	_, err = client.CallContract(testContext, ethereum.CallMsg{}, requestBlockNumber)
	require.NoError(t, err)
}

func TestNoMetricsE2ETest_HeightBasedRouting(t *testing.T) {
	if !proxyServiceHeightBasedRouting {
		t.Skip("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED is false. skipping height-based routing e2e test")
	}

	rpc, err := rpc.Dial(proxyServiceURL)
	require.NoError(t, err)

	testCases := []struct {
		name        string
		method      string
		params      []interface{}
		expectRoute string
	}{
		{
			name:        "request for non-latest height -> default",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"0x15", false}, // block 21 is beyond shards
			expectRoute: service.ResponseBackendDefault,
		},
		{
			name:        "request for height in 1st shard -> shard",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"0x2", false}, // block 2
			expectRoute: service.ResponseBackendShard,
		},
		{
			name:        "request for height in 2nd shard -> shard",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"0xF", false}, // block 15
			expectRoute: service.ResponseBackendShard,
		},
		{
			name:        "request for earliest height -> 1st shard",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"earliest", false},
			expectRoute: service.ResponseBackendShard,
		},
		{
			name:        "request for latest height -> pruning",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"latest", false},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request for finalized height -> pruning",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"finalized", false},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request with empty height -> pruning",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{nil, false},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request not requiring height -> pruning",
			method:      "eth_chainId",
			params:      []interface{}{},
			expectRoute: service.ResponseBackendPruning,
		},
		{
			name:        "request by hash -> default",
			method:      "eth_getBlockByHash",
			params:      []interface{}{"0xe9bd10bc1d62b4406dd1fb3dbf3adb54f640bdb9ebbe3dd6dfc6bcc059275e54", false},
			expectRoute: service.ResponseBackendDefault,
		},
		{
			name:        "un-parseable (invalid) height -> default",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"not-a-block-tag", false},
			expectRoute: service.ResponseBackendDefault,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := rpc.Call(nil, tc.method, tc.params...)
			require.NoError(t, err)
		})
	}
}

func TestNoMetricsE2ETestCachingMdwWithBlockNumberParam(t *testing.T) {
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getBlockByNumber",
			params:  []interface{}{"0x1", true},
			keysNum: 1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// test cache MISS and cache HIT scenarios for specified method
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			// check that cached and non-cached responses are equal

			// eth_getBlockByNumber - cache MISS
			cacheMissResp := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, cacheMissResp.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(cacheMissResp.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			expectedKey := "local-chain:evm-request:eth_getBlockByNumber:sha256:d08b426164eacf6646fb1817403ec0af5d37869a0f32a01ebfab3096fa4999be"
			containsKey(t, redisClient, expectedKey)
			// don't check CORs because proxy only force-sets header for cache hits.

			// eth_getBlockByNumber - cache HIT
			cacheHitResp := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheHitHeaderValue, cacheHitResp.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(cacheHitResp.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			containsKey(t, redisClient, expectedKey)

			// check that response bodies are the same
			require.JSONEq(t, string(body1), string(body2), "blocks should be the same")

			// check that response headers are the same
			equalHeaders(t, cacheMissResp.Header, cacheHitResp.Header)

			// check that CORS headers are present for cache hit scenario
			require.Equal(t, cacheHitResp.Header[accessControlAllowOriginHeaderName], []string{"*"})

			// eth_getBlockByNumber for request with different id - cache HIT
			diffIdResp := mkJsonRpcRequest(t, proxyServiceURL, "a string id!", tc.method, tc.params)
			require.Equal(t, cachemdw.CacheHitHeaderValue, cacheHitResp.Header[cachemdw.CacheHeaderKey][0])
			body3, err := io.ReadAll(diffIdResp.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body3)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			containsKey(t, redisClient, expectedKey)

			// check that response bodies are the same, except the id matches the request
			expectedRes := strings.Replace(string(body1), "\"id\":1", "\"id\":\"a string id!\"", 1)
			require.JSONEq(t, expectedRes, string(body3), "blocks should be the same")

			// check that response headers are the same
			equalHeaders(t, cacheMissResp.Header, diffIdResp.Header)

			// check that CORS headers are present for cache hit scenario
			require.Equal(t, diffIdResp.Header[accessControlAllowOriginHeaderName], []string{"*"})
		})
	}

	// test cache MISS and cache HIT scenarios for eth_getBlockByNumber method
	// check that cached and non-cached responses are equal
	{
		// eth_getBlockByNumber - cache MISS
		block1, err := client.BlockByNumber(testContext, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 2)
		expectedKey := "local-chain:evm-request:eth_getBlockByNumber:sha256:0bfa7c5affc525ed731803c223042b4b1eb16ee7a6a539ae213b47a3ef6e3a7d"
		containsKey(t, redisClient, expectedKey)

		// eth_getBlockByNumber - cache HIT
		block2, err := client.BlockByNumber(testContext, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 2)
		containsKey(t, redisClient, expectedKey)

		require.Equal(t, block1, block2, "blocks should be the same")
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwWithBlockNumberParam_Metrics(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getBlockByNumber",
			params:  []interface{}{"0x1", true},
			keysNum: 1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// test cache MISS and cache HIT scenarios for specified method
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			// check that cached and non-cached responses are equal

			// eth_getBlockByNumber - cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			expectedKey := "local-chain:evm-request:eth_getBlockByNumber:sha256:d08b426164eacf6646fb1817403ec0af5d37869a0f32a01ebfab3096fa4999be"
			containsKey(t, redisClient, expectedKey)

			// eth_getBlockByNumber - cache HIT
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheHitHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			containsKey(t, redisClient, expectedKey)

			require.JSONEq(t, string(body1), string(body2), "blocks should be the same")
		})
	}

	// test cache MISS and cache HIT scenarios for eth_getBlockByNumber method
	// check that cached and non-cached responses are equal
	{
		// eth_getBlockByNumber - cache MISS
		block1, err := client.BlockByNumber(testContext, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 2)
		expectedKey := "local-chain:evm-request:eth_getBlockByNumber:sha256:0bfa7c5affc525ed731803c223042b4b1eb16ee7a6a539ae213b47a3ef6e3a7d"
		containsKey(t, redisClient, expectedKey)

		// eth_getBlockByNumber - cache HIT
		block2, err := client.BlockByNumber(testContext, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 2)
		containsKey(t, redisClient, expectedKey)

		require.Equal(t, block1, block2, "blocks should be the same")
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwWithBlockNumberParam_EmptyResult(t *testing.T) {
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getTransactionCount",
			params:  []interface{}{testAddress, "0x1"},
			keysNum: 0,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// both calls should lead to cache MISS scenario, because empty results aren't cached
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			// check that responses are equal

			// eth_getBlockByNumber - cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			// eth_getBlockByNumber - cache MISS again (empty results aren't cached)
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			require.JSONEq(t, string(body1), string(body2), "blocks should be the same")
		})
	}

	// both calls should lead to cache MISS scenario, because empty results aren't cached
	// check that responses are equal
	{
		// eth_getTransactionCount - cache MISS
		bal1, err := client.NonceAt(testContext, testAddress, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 0)

		// eth_getTransactionCount - cache MISS again (empty results aren't cached)
		bal2, err := client.NonceAt(testContext, testAddress, big.NewInt(2))
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 0)

		require.Equal(t, bal1, bal2, "balances should be the same")
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwWithBlockNumberParam_ErrorResult(t *testing.T) {
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getBalance",
			params:  []interface{}{testAddress, "0x3B9ACA00"}, // block # 1000_000_000, which doesn't exist
			keysNum: 0,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// both calls should lead to cache MISS scenario, because error results aren't cached
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			//
			// cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.Error(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)

			// cache MISS again (error results aren't cached)
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.Error(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
		})
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwWithBlockNumberParam_FutureBlocks(t *testing.T) {
	futureBlockNumber := "0x3B9ACA00" // block # 1000_000_000, which doesn't exist
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc     string
		method   string
		params   []interface{}
		keysNum  int
		errorMsg string
	}{
		{
			desc:     "test case #1",
			method:   "eth_getBalance",
			params:   []interface{}{testAddress, futureBlockNumber},
			keysNum:  0,
			errorMsg: "height 1000000000 must be less than or equal to the current blockchain height",
		},
		{
			desc:     "test case #2",
			method:   "eth_getStorageAt",
			params:   []interface{}{testAddress, "0x6661e9d6d8b923d5bbaab1b96e1dd51ff6ea2a93520fdc9eb75d059238b8c5e9", futureBlockNumber},
			keysNum:  0,
			errorMsg: "invalid height: cannot query with height in the future; please provide a valid height",
		},
		{
			desc:     "test case #3",
			method:   "eth_getTransactionCount",
			params:   []interface{}{testAddress, futureBlockNumber},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #4",
			method:   "eth_getBlockTransactionCountByNumber",
			params:   []interface{}{futureBlockNumber},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #5",
			method:   "eth_getUncleCountByBlockNumber",
			params:   []interface{}{futureBlockNumber},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #6",
			method:   "eth_getCode",
			params:   []interface{}{testAddress, futureBlockNumber},
			keysNum:  0,
			errorMsg: "invalid height: cannot query with height in the future; please provide a valid height",
		},
		{
			desc:     "test case #7",
			method:   "eth_getBlockByNumber",
			params:   []interface{}{futureBlockNumber, false},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #8",
			method:   "eth_getTransactionByBlockNumberAndIndex",
			params:   []interface{}{futureBlockNumber, "0x0"},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #9",
			method:   "eth_getUncleByBlockNumberAndIndex",
			params:   []interface{}{futureBlockNumber, "0x0"},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #10",
			method:   "eth_call",
			params:   []interface{}{struct{}{}, futureBlockNumber},
			keysNum:  0,
			errorMsg: "header not found",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// both calls should lead to cache MISS scenario, because error results aren't cached
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			//
			// cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			if tc.errorMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			}
			expectKeysNum(t, redisClient, tc.keysNum)

			// cache MISS again (error results aren't cached)
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			if tc.errorMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			}
			expectKeysNum(t, redisClient, tc.keysNum)
		})
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwWithBlockHashParam_UnexistingBlockHashes(t *testing.T) {
	unexistingBlockHash := "0xb903239f8543d04b5dc1ba6579132b143087c68db1b2168786408fcbce568238"

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc     string
		method   string
		params   []interface{}
		keysNum  int
		errorMsg string
	}{
		{
			desc:     "test case #1",
			method:   "eth_getBlockTransactionCountByHash",
			params:   []interface{}{unexistingBlockHash},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #2",
			method:   "eth_getUncleCountByBlockHash",
			params:   []interface{}{unexistingBlockHash},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #3",
			method:   "eth_getBlockByHash",
			params:   []interface{}{unexistingBlockHash, false},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #4",
			method:   "eth_getUncleByBlockHashAndIndex",
			params:   []interface{}{unexistingBlockHash, "0x0"},
			keysNum:  0,
			errorMsg: "",
		},
		{
			desc:     "test case #5",
			method:   "eth_getTransactionByBlockHashAndIndex",
			params:   []interface{}{unexistingBlockHash, "0x0"},
			keysNum:  0,
			errorMsg: "",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// both calls should lead to cache MISS scenario, because error results aren't cached
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			//
			// cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			if tc.errorMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			}
			expectKeysNum(t, redisClient, tc.keysNum)

			// cache MISS again (error results aren't cached)
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			if tc.errorMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			}
			expectKeysNum(t, redisClient, tc.keysNum)
		})
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwWithBlockNumberParam_DiffJsonRpcReqIDs(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc    string
		method  string
		params  []interface{}
		keysNum int
	}{
		{
			desc:    "test case #1",
			method:  "eth_getBlockByNumber",
			params:  []interface{}{"0x1", true},
			keysNum: 1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// test cache MISS and cache HIT scenarios for specified method
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			// NOTE: JSON-RPC request IDs are different
			// check that cached and non-cached responses differ only in response ID

			// eth_getBlockByNumber - cache MISS
			resp1 := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, resp1.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(resp1.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			expectedKey := "local-chain:evm-request:eth_getBlockByNumber:sha256:d08b426164eacf6646fb1817403ec0af5d37869a0f32a01ebfab3096fa4999be"
			containsKey(t, redisClient, expectedKey)

			// eth_getBlockByNumber - cache HIT
			resp2 := mkJsonRpcRequest(t, proxyServiceURL, 2, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheHitHeaderValue, resp2.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(resp2.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			containsKey(t, redisClient, expectedKey)

			rpcResp1, err := cachemdw.UnmarshalJsonRpcResponse(body1)
			require.NoError(t, err)
			rpcResp2, err := cachemdw.UnmarshalJsonRpcResponse(body2)
			require.NoError(t, err)

			// JSON-RPC Version and Result should be equal
			require.Equal(t, rpcResp1.Version, rpcResp2.Version)
			require.Equal(t, rpcResp1.Result, rpcResp2.Result)

			// JSON-RPC response ID should correspond to JSON-RPC request ID
			require.Equal(t, string(rpcResp1.ID), "1")
			require.Equal(t, string(rpcResp2.ID), "2")

			// JSON-RPC error should be empty
			require.Empty(t, rpcResp1.JsonRpcError)
			require.Empty(t, rpcResp2.JsonRpcError)

			// Double-check that JSON-RPC responses differ only in response ID
			rpcResp2.ID = []byte("1")
			require.Equal(t, rpcResp1, rpcResp2)
		})
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwForStaticMethods(t *testing.T) {
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	for _, tc := range []struct {
		desc        string
		method      string
		params      []interface{}
		keysNum     int
		expectedKey string
	}{
		{
			desc:        "test case #1",
			method:      "eth_chainId",
			params:      []interface{}{},
			keysNum:     1,
			expectedKey: "local-chain:evm-request:eth_chainId:sha256:*",
		},
		{
			desc:        "test case #2",
			method:      "net_version",
			params:      []interface{}{},
			keysNum:     2,
			expectedKey: "local-chain:evm-request:net_version:sha256:*",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// test cache MISS and cache HIT scenarios for specified method
			// check corresponding values in cachemdw.CacheHeaderKey HTTP header
			// check that cached and non-cached responses are equal

			// cache MISS
			cacheMissResp := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheMissHeaderValue, cacheMissResp.Header[cachemdw.CacheHeaderKey][0])
			body1, err := io.ReadAll(cacheMissResp.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body1)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			containsKey(t, redisClient, tc.expectedKey)

			// cache HIT
			cacheHitResp := mkJsonRpcRequest(t, proxyServiceURL, 1, tc.method, tc.params)
			require.Equal(t, cachemdw.CacheHitHeaderValue, cacheHitResp.Header[cachemdw.CacheHeaderKey][0])
			body2, err := io.ReadAll(cacheHitResp.Body)
			require.NoError(t, err)
			err = checkJsonRpcErr(body2)
			require.NoError(t, err)
			expectKeysNum(t, redisClient, tc.keysNum)
			containsKey(t, redisClient, tc.expectedKey)

			// check that response bodies are the same
			require.JSONEq(t, string(body1), string(body2), "blocks should be the same")

			// check that response headers are the same
			equalHeaders(t, cacheMissResp.Header, cacheHitResp.Header)

			// check that CORS headers are present for cache hit scenario
			require.Equal(t, cacheHitResp.Header[accessControlAllowOriginHeaderName], []string{"*"})
		})
	}

	cleanUpRedis(t, redisClient)
	// test cache MISS and cache HIT scenarios for eth_chainId method
	// check that cached and non-cached responses are equal
	{
		// eth_getBlockByNumber - cache MISS
		block1, err := client.ChainID(testContext)
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 1)
		expectedKey := "local-chain:evm-request:eth_chainId:sha256:*"
		containsKey(t, redisClient, expectedKey)

		// eth_getBlockByNumber - cache HIT
		block2, err := client.ChainID(testContext)
		require.NoError(t, err)
		expectKeysNum(t, redisClient, 1)
		containsKey(t, redisClient, expectedKey)

		require.Equal(t, block1, block2, "blocks should be the same")
	}

	cleanUpRedis(t, redisClient)
}

func TestNoMetricsE2ETestCachingMdwForGetTxByHashMethod(t *testing.T) {
	// create api and database clients
	evmClient, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	addr := common.HexToAddress(evmFaucetAddressHex)
	balance, err := evmClient.BalanceAt(testContext, addr, nil)
	if err != nil {
		log.Fatalf("can't get balance for evm faucet: %v\n", err)
	}
	require.NotEqual(t, "0", balance.String())

	addressToFund := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	// submit eth tx
	tx := fundEVMAddress(t, evmClient, addressToFund)
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	expectedKey := "local-chain:evm-request:eth_getTransactionByHash:sha256:*"
	// getting tx by hash in the loop until JSON-RPC response result won't be null
	// NOTE: it's Cache Miss scenario, because we don't cache null responses
	waitUntilTxAppearsInMempool(t, tx.Hash())
	expectKeysNum(t, redisClient, 0)
	// getting tx by hash in the loop until JSON-RPC response result won't indicate that tx included in block
	// NOTE: it's Cache Miss scenario, because we don't cache txs which is in mempool
	cacheMissBody, cacheMissHeaders := getTxByHashFromBlock(t, tx.Hash(), cachemdw.CacheMissHeaderValue)
	expectKeysNum(t, redisClient, 1)
	containsKey(t, redisClient, expectedKey)
	// on previous step we already got tx which is included in block, so calling this again triggers Cache Hit scenario
	cacheHitBody, cacheHitHeaders := getTxByHashFromBlock(t, tx.Hash(), cachemdw.CacheHitHeaderValue)
	expectKeysNum(t, redisClient, 1)
	containsKey(t, redisClient, expectedKey)

	// check that response bodies are the same
	require.JSONEq(t, string(cacheMissBody), string(cacheHitBody), "blocks should be the same")

	// check that response headers are the same
	equalHeaders(t, cacheMissHeaders, cacheHitHeaders)

	// check that CORS headers are present for cache hit scenario
	require.Equal(t, cacheHitHeaders[accessControlAllowOriginHeaderName], []string{"*"})
}

func TestNoMetricsE2ETestCachingMdwForGetTxReceiptByHashMethod(t *testing.T) {
	// create api and database clients
	evmClient, err := ethclient.Dial(proxyServiceURL)
	if err != nil {
		t.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	addr := common.HexToAddress(evmFaucetAddressHex)
	balance, err := evmClient.BalanceAt(testContext, addr, nil)
	if err != nil {
		log.Fatalf("can't get balance for evm faucet: %v\n", err)
	}
	require.NotEqual(t, "0", balance.String())

	addressToFund := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	// submit eth tx
	tx := fundEVMAddress(t, evmClient, addressToFund)
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)

	expectedKey := "local-chain:evm-request:eth_getTransactionReceipt:sha256:*"
	// getting tx receipt by hash in the loop until JSON-RPC response result won't be null
	// it's Cache Miss scenario, because we don't cache null responses
	// NOTE: eth_getTransactionReceipt returns null JSON-RPC response result for txs in mempool, so at this point
	// tx already included in block
	cacheMissBody, cacheMissHeaders := getTxReceiptByHash(t, tx.Hash(), cachemdw.CacheMissHeaderValue)
	expectKeysNum(t, redisClient, 1)
	containsKey(t, redisClient, expectedKey)
	// on previous step we already got tx which is included in block, so calling this again triggers Cache Hit scenario
	cacheHitBody, cacheHitHeaders := getTxReceiptByHash(t, tx.Hash(), cachemdw.CacheHitHeaderValue)
	expectKeysNum(t, redisClient, 1)
	containsKey(t, redisClient, expectedKey)

	// check that response bodies are the same
	require.JSONEq(t, string(cacheMissBody), string(cacheHitBody), "blocks should be the same")

	// check that response headers are the same
	equalHeaders(t, cacheMissHeaders, cacheHitHeaders)

	// check that CORS headers are present for cache hit scenario
	require.Equal(t, cacheHitHeaders[accessControlAllowOriginHeaderName], []string{"*"})
}
