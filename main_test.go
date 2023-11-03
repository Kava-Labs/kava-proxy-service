package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
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
// NOTE: it completely ignores presence/absence of cachemdw.CacheHeaderKey,
// it's done in that way to allow comparison of headers for cache miss and cache hit cases
// also it ignores presence/absence of CORS headers
func containsHeaders(t *testing.T, headersMap1, headersMap2 http.Header) {
	for name, value := range headersMap1 {
		if name == cachemdw.CacheHeaderKey || name == "Server" || name == accessControlAllowOriginHeaderName {
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
	Jsonrpc string      `json:"jsonrpc"`
	Id      int         `json:"id"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error"`
}

func checkJsonRpcErr(body []byte) error {
	var resp jsonRpcResponse
	err := json.Unmarshal(body, &resp)
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return errors.New(resp.Error)
	}

	if resp.Result == "" {
		return errors.New("result is empty")
	}

	return nil
}
