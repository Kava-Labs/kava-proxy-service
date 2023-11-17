package main_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

const (
	EthClientUserAgent = "Go-http-client/1.1"

	accessControlAllowOriginHeaderName = "Access-Control-Allow-Origin"

	evmFaucetPrivateKeyHex = "247069f0bc3a5914cb2fd41e4133bbdaa6dbed9f47a01b9f110b5602c6e4cdd9"
	evmFaucetAddressHex    = "0x6767114FFAA17c6439D7aEA480738b982ce63A02"
)

var (
	testContext = context.Background()

	testServiceLogger = func() logging.ServiceLogger {
		logger, err := logging.New("ERROR")
		if err != nil {
			panic(err)

		}
		return logger
	}()

	proxyServiceURL        = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_URL")
	proxyServiceHostname   = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_HOSTNAME")
	proxyServicePruningURL = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_PRUNING_URL")

	proxyServiceHeightBasedRouting, _ = strconv.ParseBool(os.Getenv("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED"))

	databaseURL      = os.Getenv("TEST_DATABASE_ENDPOINT_URL")
	databasePassword = os.Getenv("DATABASE_PASSWORD")
	databaseUsername = os.Getenv("DATABASE_USERNAME")
	databaseName     = os.Getenv("DATABASE_NAME")
	databaseConfig   = database.PostgresDatabaseConfig{
		DatabaseName:          databaseName,
		DatabaseEndpointURL:   databaseURL,
		DatabaseUsername:      databaseUsername,
		DatabasePassword:      databasePassword,
		SSLEnabled:            false,
		QueryLoggingEnabled:   false,
		Logger:                &testServiceLogger,
		RunDatabaseMigrations: false,
	}

	redisURL      = os.Getenv("TEST_REDIS_ENDPOINT_URL")
	redisPassword = os.Getenv("REDIS_PASSWORD")
)

// lookup all the request metrics in the database paging as necessary
// search for any request metrics during the time window for particular request methods
func findMetricsInWindowForMethods(db database.PostgresClient, startTime time.Time, endTime time.Time, testedmethods []string) []database.ProxiedRequestMetric {
	var nextCursor int64
	var proxiedRequestMetrics []database.ProxiedRequestMetric

	proxiedRequestMetricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(testContext, db.DB, nextCursor, 10000)
	if err != nil {
		panic(err)
	}

	proxiedRequestMetrics = proxiedRequestMetricsPage
	for nextCursor != 0 {
		proxiedRequestMetricsPage, nextCursor, err = database.ListProxiedRequestMetricsWithPagination(testContext, db.DB, nextCursor, 10000)
		if err != nil {
			panic(err)
		}

		proxiedRequestMetrics = append(proxiedRequestMetrics, proxiedRequestMetricsPage...)
	}

	var requestMetricsDuringRequestWindow []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most recent request metrics first
	for i := len(proxiedRequestMetrics) - 1; i >= 0; i-- {
		requestMetric := proxiedRequestMetrics[i]
		if requestMetric.RequestTime.After(startTime) && requestMetric.RequestTime.Before(endTime) {
			for _, testedMethod := range testedmethods {
				if requestMetric.MethodName == testedMethod {
					requestMetricsDuringRequestWindow = append(requestMetricsDuringRequestWindow, requestMetric)
				}
			}
		}
	}

	return requestMetricsDuringRequestWindow
}

func TestE2ETestProxyReturnsNonZeroLatestBlockHeader(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	header, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	assert.Greater(t, int(header.Number.Int64()), 0)
}

func TestE2ETestProxyProxiesForMultipleHosts(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	header, err := client.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	assert.Greater(t, int(header.Number.Int64()), 0)

	pruningClient, err := ethclient.Dial(proxyServicePruningURL)

	require.NoError(t, err)

	header, err = pruningClient.HeaderByNumber(testContext, nil)
	require.NoError(t, err)

	assert.Greater(t, int(header.Number.Int64()), 0)
}

func TestE2ETestProxyCreatesRequestMetricForEachRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	_, err = client.HeaderByNumber(testContext, nil)

	endTime := time.Now()

	require.NoError(t, err)

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, []string{testEthMethodName})

	assert.Greater(t, len(requestMetricsDuringRequestWindow), 0)

	requestMetricDuringRequestWindow := requestMetricsDuringRequestWindow[0]

	assert.Greater(t, requestMetricDuringRequestWindow.ResponseLatencyMilliseconds, int64(0))
	assert.Equal(t, requestMetricDuringRequestWindow.MethodName, testEthMethodName)
	assert.Equal(t, requestMetricDuringRequestWindow.Hostname, proxyServiceHostname)
	assert.NotEqual(t, requestMetricDuringRequestWindow.RequestIP, "")
	assert.Equal(t, *requestMetricDuringRequestWindow.UserAgent, EthClientUserAgent)
	assert.NotEqual(t, *requestMetricDuringRequestWindow.Referer, "")
	assert.NotEqual(t, *requestMetricDuringRequestWindow.Origin, "")
}

func TestE2ETestProxyTracksBlockNumberForEth_getBlockByNumberRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	response, err := client.HeaderByNumber(testContext, nil)

	require.NoError(t, err)

	requestBlockNumber := response.Number

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	_, err = client.HeaderByNumber(testContext, requestBlockNumber)

	endTime := time.Now()

	require.NoError(t, err)

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, []string{testEthMethodName})

	assert.Greater(t, len(requestMetricsDuringRequestWindow), 0)
	requestMetricDuringRequestWindow := requestMetricsDuringRequestWindow[0]

	assert.Equal(t, requestMetricDuringRequestWindow.MethodName, testEthMethodName)
	assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
	assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber.Int64())
}

func TestE2ETestProxyTracksBlockTagForEth_getBlockByNumberRequest(t *testing.T) {
	testEthMethodName := "eth_getBlockByNumber"
	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// make request to api and track start / end time of the request to
	startTime := time.Now()

	// will default to latest
	_, err = client.HeaderByNumber(testContext, nil)

	endTime := time.Now()

	require.NoError(t, err)

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, []string{testEthMethodName})

	assert.Greater(t, len(requestMetricsDuringRequestWindow), 0)
	requestMetricDuringRequestWindow := requestMetricsDuringRequestWindow[0]

	assert.Equal(t, requestMetricDuringRequestWindow.MethodName, testEthMethodName)
	assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
	assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, decode.BlockTagToNumberCodec["latest"])
}

func TestE2ETestProxyTracksBlockNumberForMethodsWithBlockNumberParam(t *testing.T) {
	testedmethods := decode.CacheableByBlockNumberMethods
	testRandomAddressHex := "0x6767114FFAA17C6439D7AEA480738B982CE63A02"
	testAddress := common.HexToAddress(testRandomAddressHex)
	testRandomHash := common.HexToHash(testRandomAddressHex)

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)
	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)
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
	startTime := time.Now()

	// eth_getBalance
	_, _ = client.BalanceAt(testContext, testAddress, requestBlockNumber)

	// eth_getStorageAt
	_, _ = client.StorageAt(testContext, testAddress, testRandomHash, requestBlockNumber)

	// eth_getTransactionCount
	_, _ = client.NonceAt(testContext, testAddress, requestBlockNumber)

	// eth_getBlockTransactionCountByNumber
	_, _ = client.PendingTransactionCount(testContext)

	// eth_getCode
	_, _ = client.CodeAt(testContext, testAddress, requestBlockNumber)

	// eth_getBlockByNumber
	_, _ = client.HeaderByNumber(testContext, requestBlockNumber)

	// eth_call
	_, _ = client.CallContract(testContext, ethereum.CallMsg{}, requestBlockNumber)

	// plus a buffer for slower connections (amd64 lol) :)
	endTime := time.Now().Add(10)

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, testedmethods)

	// assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), len(testedmethods))
	// should be the above but geth doesn't implement client methods for all of them
	assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), 7)

	for _, requestMetricDuringRequestWindow := range requestMetricsDuringRequestWindow {
		assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
		if requestMetricDuringRequestWindow.MethodName == "eth_getBlockTransactionCountByNumber" {
			assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, decode.BlockTagToNumberCodec["pending"])
			continue
		}
		assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber.Int64())
	}
}

func TestE2ETestProxyTracksBlockNumberForMethodsWithBlockHashParam(t *testing.T) {
	testedmethods := decode.CacheableByBlockHashMethods

	// create api and database clients
	client, err := ethclient.Dial(proxyServiceURL)

	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)

	require.NoError(t, err)

	// get the latest queryable block number
	// need to do this dynamically since not all blocks
	// are queryable for a given network
	latestBlock, err := client.HeaderByNumber(testContext, nil)

	require.NoError(t, err)

	requestBlockHash := latestBlock.ParentHash
	// minus one since we are looking up the parent block
	requestBlockNumber := latestBlock.Number.Int64() - 1

	// make requests to api and track start / end time of the request
	// we don't check response errors because the proxy will create metrics
	// for each request whether the kava node api returns an error or not
	// and if it doesn't the test itself will fail due to missing metrics
	startTime := time.Now()

	// eth_getBlockByHash
	_, _ = client.BlockByHash(testContext, requestBlockHash)

	// eth_getBlockTransactionCountByHash
	_, _ = client.TransactionCount(testContext, requestBlockHash)

	// eth_getTransactionByBlockHashAndIndex
	_, _ = client.TransactionInBlock(testContext, requestBlockHash, 0)
	endTime := time.Now()

	requestMetricsDuringRequestWindow := findMetricsInWindowForMethods(databaseClient, startTime, endTime, testedmethods)

	// assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), len(testedmethods))
	// should be the above but geth doesn't implement client methods for all of them
	assert.GreaterOrEqual(t, len(requestMetricsDuringRequestWindow), 3)

	for _, requestMetricDuringRequestWindow := range requestMetricsDuringRequestWindow {
		assert.NotNil(t, *requestMetricDuringRequestWindow.BlockNumber)
		assert.Equal(t, *requestMetricDuringRequestWindow.BlockNumber, requestBlockNumber)
	}
}

func TestE2ETest_HeightBasedRouting(t *testing.T) {
	if !proxyServiceHeightBasedRouting {
		t.Skip("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED is false. skipping height-based routing e2e test")
	}

	rpc, err := rpc.Dial(proxyServiceURL)
	require.NoError(t, err)

	databaseClient, err := database.NewPostgresClient(databaseConfig)
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
			params:      []interface{}{"0x2", false},
			expectRoute: service.ResponseBackendDefault,
		},
		{
			name:        "request for earliest height -> default",
			method:      "eth_getBlockByNumber",
			params:      []interface{}{"earliest", false},
			expectRoute: service.ResponseBackendDefault,
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
			startTime := time.Now()
			err := rpc.Call(nil, tc.method, tc.params...)
			require.NoError(t, err)

			metrics := findMetricsInWindowForMethods(databaseClient, startTime, time.Now(), []string{tc.method})

			require.Len(t, metrics, 1)
			fmt.Printf("%+v\n", metrics[0])
			require.Equal(t, metrics[0].MethodName, tc.method)
			require.Equal(t, metrics[0].ResponseBackend, tc.expectRoute)
		})
	}
}

func TestE2ETestCachingMdwWithBlockNumberParam(t *testing.T) {
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

// equalHeaders checks that headers of headersMap1 and headersMap2 are equal
// NOTE: it completely ignores presence/absence of cachemdw.CacheHeaderKey,
// it's done in that way to allow comparison of headers for cache miss and cache hit cases
// also it ignores presence/absence of CORS headers
func equalHeaders(t *testing.T, headersMap1, headersMap2 http.Header) {
	containsHeaders(t, headersMap1, headersMap2)
	containsHeaders(t, headersMap2, headersMap1)
}

// containsHeaders checks that headersMap1 contains all headers from headersMap2 and that values for headers are the same
// it completely ignores presence/absence of cachemdw.CacheHeaderKey,
// it's done in that way to allow comparison of headers for cache miss and cache hit cases
// it ignores presence/absence of CORS headers,
// it's because caching layer forcefully sets CORS headers in cache hit scenario, even if they didn't exist before
// we skip Date header, because time between requests can change a bit, and we don't want random test fails due to this
// we skip Server header because it's not included in our allow list for headers, consult .env.WHITELISTED_HEADERS for allow list
func containsHeaders(t *testing.T, headersMap1, headersMap2 http.Header) {
	headersToSkip := map[string]struct{}{
		cachemdw.CacheHeaderKey:            {},
		accessControlAllowOriginHeaderName: {},
		"Date":                             {},
		"Server":                           {},
	}

	for name, value := range headersMap1 {
		_, skip := headersToSkip[name]
		if skip {
			continue
		}

		require.Equal(t, value, headersMap2[name])
	}
}

func TestE2ETestCachingMdwWithBlockNumberParam_Metrics(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)
	require.NoError(t, err)
	db, err := database.NewPostgresClient(databaseConfig)
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       0,
	})
	cleanUpRedis(t, redisClient)
	expectKeysNum(t, redisClient, 0)
	// startTime is a time before first request
	startTime := time.Now()

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

	// endTime is a time after last request
	endTime := time.Now()
	// get metrics within [startTime, endTime] time range for eth_getBlockByNumber requests
	allMetrics := getAllMetrics(context.Background(), t, db)
	filteredMetrics := filterMetrics(allMetrics, []string{"eth_getBlockByNumber"}, startTime, endTime)

	// we expect 4 metrics, 2 of them are cache hits and two of them are cache misses
	require.Len(t, filteredMetrics, 4)
	cacheHits := 0
	cacheMisses := 0
	for _, metric := range filteredMetrics {
		if metric.CacheHit {
			cacheHits++
		} else {
			cacheMisses++
		}
	}
	require.Equal(t, 2, cacheHits)
	require.Equal(t, 2, cacheMisses)

	cleanUpRedis(t, redisClient)
}

// getAllMetrics gets all metrics from database
func getAllMetrics(ctx context.Context, t *testing.T, db database.PostgresClient) []database.ProxiedRequestMetric {
	var (
		metrics        []database.ProxiedRequestMetric
		cursor         int64
		limit          int  = 10_000
		firstIteration bool = true
	)

	for firstIteration || cursor != 0 {
		metricsPage, nextCursor, err := database.ListProxiedRequestMetricsWithPagination(
			ctx,
			db.DB,
			cursor,
			limit,
		)
		require.NoError(t, err)

		metrics = append(metrics, metricsPage...)
		cursor = nextCursor
		firstIteration = false
	}

	return metrics
}

// filterMetrics filters metrics based on time range and method
func filterMetrics(
	metrics []database.ProxiedRequestMetric,
	methods []string,
	startTime time.Time,
	endTime time.Time,
) []database.ProxiedRequestMetric {
	var metricsWithinTimerange []database.ProxiedRequestMetric
	// iterate in reverse order to start checking the most recent metrics first
	for i := len(metrics) - 1; i >= 0; i-- {
		metric := metrics[i]
		ok1 := metric.RequestTime.After(startTime) && metric.RequestTime.Before(endTime)
		ok2 := contains(methods, metric.MethodName)
		if ok1 && ok2 {
			metricsWithinTimerange = append(metricsWithinTimerange, metric)
		}
	}

	return metricsWithinTimerange
}

func contains(items []string, item string) bool {
	for _, nextItem := range items {
		if item == nextItem {
			return true
		}
	}

	return false
}

func TestE2ETestCachingMdwWithBlockNumberParam_EmptyResult(t *testing.T) {
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

func TestE2ETestCachingMdwWithBlockNumberParam_ErrorResult(t *testing.T) {
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

func TestE2ETestCachingMdwWithBlockNumberParam_FutureBlocks(t *testing.T) {
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

func TestE2ETestCachingMdwWithBlockHashParam_UnexistingBlockHashes(t *testing.T) {
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

func TestE2ETestCachingMdwWithBlockNumberParam_DiffJsonRpcReqIDs(t *testing.T) {
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

func expectKeysNum(t *testing.T, redisClient *redis.Client, keysNum int) {
	keys, err := redisClient.Keys(context.Background(), "*").Result()
	require.NoError(t, err)

	require.Equal(t, keysNum, len(keys))
}

func containsKey(t *testing.T, redisClient *redis.Client, key string) {
	keys, err := redisClient.Keys(context.Background(), key).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(keys), 1)
}

func cleanUpRedis(t *testing.T, redisClient *redis.Client) {
	keys, err := redisClient.Keys(context.Background(), "*").Result()
	require.NoError(t, err)

	if len(keys) != 0 {
		_, err = redisClient.Del(context.Background(), keys...).Result()
		require.NoError(t, err)
	}
}

func mkJsonRpcRequest(t *testing.T, proxyServiceURL string, id int, method string, params []interface{}) *http.Response {
	req := newJsonRpcRequest(id, method, params)
	reqInJSON, err := json.Marshal(req)
	require.NoError(t, err)
	reqReader := bytes.NewBuffer(reqInJSON)
	resp, err := http.Post(proxyServiceURL, "application/json", reqReader)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return resp
}

type jsonRpcRequest struct {
	JsonRpc string        `json:"jsonrpc"`
	Id      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

func newJsonRpcRequest(id int, method string, params []interface{}) *jsonRpcRequest {
	return &jsonRpcRequest{
		JsonRpc: "2.0",
		Id:      id,
		Method:  method,
		Params:  params,
	}
}

type jsonRpcResponse struct {
	Jsonrpc      string        `json:"jsonrpc"`
	Id           int           `json:"id"`
	Result       interface{}   `json:"result"`
	JsonRpcError *jsonRpcError `json:"error,omitempty"`
}

type jsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// String returns the string representation of the error
func (e *jsonRpcError) String() string {
	return fmt.Sprintf("%s (code: %d)", e.Message, e.Code)
}

func checkJsonRpcErr(body []byte) error {
	var resp jsonRpcResponse
	err := json.Unmarshal(body, &resp)
	if err != nil {
		return err
	}

	if resp.JsonRpcError != nil {
		return errors.New(resp.JsonRpcError.String())
	}

	if resp.Result == "" {
		return errors.New("result is empty")
	}

	return nil
}

func TestE2ETestCachingMdwForStaticMethods(t *testing.T) {
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

func TestE2ETestCachingMdwForGetTxByHashMethod(t *testing.T) {
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

// waitUntilTxAppearsInMempool gets tx by hash in the loop until JSON-RPC response result won't be null
// also it checks that it always cache miss scenario
func waitUntilTxAppearsInMempool(t *testing.T, hash common.Hash) {
	err := backoff.Retry(func() error {
		method := "eth_getTransactionByHash"
		params := []interface{}{hash}
		cacheMissResp := mkJsonRpcRequest(t, proxyServiceURL, 1, method, params)
		require.Equal(t, cachemdw.CacheMissHeaderValue, cacheMissResp.Header[cachemdw.CacheHeaderKey][0])
		body, err := io.ReadAll(cacheMissResp.Body)
		require.NoError(t, err)
		err = checkJsonRpcErr(body)
		require.NoError(t, err)

		var tx getTxByHashResponse
		err = json.Unmarshal(body, &tx)
		require.NoError(t, err)

		if tx.Result == nil {
			return errors.New("tx is not found")
		}

		return nil
	}, backoff.NewConstantBackOff(time.Millisecond*10))
	require.NoError(t, err)
}

// getTxByHashFromBlock gets tx by hash in the loop until JSON-RPC response result won't indicate that tx included in block
func getTxByHashFromBlock(t *testing.T, hash common.Hash, expectedCacheHeaderValue string) ([]byte, http.Header) {
	var (
		body []byte
		resp *http.Response
	)
	err := backoff.Retry(func() error {
		method := "eth_getTransactionByHash"
		params := []interface{}{hash}
		resp = mkJsonRpcRequest(t, proxyServiceURL, 1, method, params)
		require.Equal(t, expectedCacheHeaderValue, resp.Header[cachemdw.CacheHeaderKey][0])
		var err error
		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = checkJsonRpcErr(body)
		require.NoError(t, err)

		var tx getTxByHashResponse
		err = json.Unmarshal(body, &tx)
		require.NoError(t, err)

		if !tx.IsIncludedInBlock() {
			return errors.New("tx is not included in block yet")
		}

		return nil
	}, backoff.NewConstantBackOff(time.Millisecond*10))
	require.NoError(t, err)

	return body, resp.Header
}

type getTxByHashResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  *struct {
		BlockHash        interface{} `json:"blockHash"`
		BlockNumber      interface{} `json:"blockNumber"`
		From             string      `json:"from"`
		Gas              string      `json:"gas"`
		GasPrice         string      `json:"gasPrice"`
		Hash             string      `json:"hash"`
		Input            string      `json:"input"`
		Nonce            string      `json:"nonce"`
		To               string      `json:"to"`
		TransactionIndex interface{} `json:"transactionIndex"`
		Value            string      `json:"value"`
		Type             string      `json:"type"`
		ChainId          string      `json:"chainId"`
		V                string      `json:"v"`
		R                string      `json:"r"`
		S                string      `json:"s"`
	} `json:"result"`
}

// IsIncludedInBlock checks if transaction included in block
// transaction included in block if block hash, block number, and tx index are not null
func (tx *getTxByHashResponse) IsIncludedInBlock() bool {
	if tx.Result == nil {
		return false
	}

	return tx.Result.BlockHash != nil &&
		tx.Result.BlockHash != "" &&
		tx.Result.BlockNumber != nil &&
		tx.Result.BlockNumber != "" &&
		tx.Result.TransactionIndex != nil &&
		tx.Result.TransactionIndex != ""
}

// fundEVMAddress sends money from evm faucet to provided address
func fundEVMAddress(t *testing.T, evmClient *ethclient.Client, addressToFund common.Address) *types.Transaction {
	privateKey, err := crypto.HexToECDSA(evmFaucetPrivateKeyHex)
	require.NoError(t, err)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal(t, "cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	require.Equal(t, common.HexToAddress(evmFaucetAddressHex), fromAddress)
	nonce, err := evmClient.PendingNonceAt(testContext, fromAddress)
	require.NoError(t, err)

	value := big.NewInt(1_000_000) // in wei (10^-18 ETH)
	gasLimit := uint64(21000)      // in units
	gasPrice, err := evmClient.SuggestGasPrice(testContext)
	require.NoError(t, err)

	var data []byte
	tx := types.NewTransaction(nonce, addressToFund, value, gasLimit, gasPrice, data)

	chainID, err := evmClient.NetworkID(testContext)
	require.NoError(t, err)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	require.NoError(t, err)

	err = evmClient.SendTransaction(testContext, signedTx)
	require.NoErrorf(t, err, "can't send tx")

	return signedTx
}

func TestE2ETestCachingMdwForGetTxReceiptByHashMethod(t *testing.T) {
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

// getting tx receipt by hash in the loop until JSON-RPC response result won't be null
// NOTE: eth_getTransactionReceipt returns null JSON-RPC response result for txs in mempool, so returned tx will be included in block
func getTxReceiptByHash(t *testing.T, hash common.Hash, expectedCacheHeaderValue string) ([]byte, http.Header) {
	var (
		body      []byte
		resp      *http.Response
		txReceipt getTxReceiptByHashResponse
	)
	err := backoff.Retry(func() error {
		method := "eth_getTransactionReceipt"
		params := []interface{}{hash}
		resp = mkJsonRpcRequest(t, proxyServiceURL, 1, method, params)
		require.Equal(t, expectedCacheHeaderValue, resp.Header[cachemdw.CacheHeaderKey][0])
		var err error
		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		err = checkJsonRpcErr(body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &txReceipt)
		require.NoError(t, err)

		if txReceipt.Result == nil {
			return errors.New("tx is not found")
		}

		return nil
	}, backoff.NewConstantBackOff(time.Millisecond*10))
	require.NoError(t, err)

	// NOTE: eth_getTransactionReceipt returns null JSON-RPC response result for txs in mempool, so returned tx must be included in block
	require.True(t, txReceipt.IsIncludedInBlock())

	return body, resp.Header
}

type getTxReceiptByHashResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  *struct {
		BlockHash         string        `json:"blockHash"`
		BlockNumber       string        `json:"blockNumber"`
		ContractAddress   interface{}   `json:"contractAddress"`
		CumulativeGasUsed string        `json:"cumulativeGasUsed"`
		From              string        `json:"from"`
		GasUsed           string        `json:"gasUsed"`
		Logs              []interface{} `json:"logs"`
		LogsBloom         string        `json:"logsBloom"`
		Status            string        `json:"status"`
		To                string        `json:"to"`
		TransactionHash   string        `json:"transactionHash"`
		TransactionIndex  string        `json:"transactionIndex"`
		Type              string        `json:"type"`
	} `json:"result"`
}

// IsIncludedInBlock checks if transaction included in block
// transaction included in block if block hash, block number, and tx index are not empty
func (tx *getTxReceiptByHashResponse) IsIncludedInBlock() bool {
	if tx.Result == nil {
		return false
	}

	return tx.Result.BlockHash != "" &&
		tx.Result.BlockNumber != "" &&
		tx.Result.TransactionIndex != ""
}
